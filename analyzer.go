package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
)

type FnType int

const (
	Unknown = 0 + iota

	// Leaf function (no calls)
	Leaf

	// Vanilla non-leaf (no morestack calls)
	NoSplit

	// Non-leaf with regular morestack call
	SplitSmall

	// Non-leaf with call to morestack_non_split
	SplitLarge
)

type astate struct {
	seenShort  bool
	seenLong   bool
	seenCall   bool
	funcs      map[string]FnType
	collisions int64
}

func (s *astate) recordFunc(fname string) {
	disp := FnType(Leaf)
	if s.seenLong {
		disp = SplitLarge
	} else if s.seenShort {
		disp = SplitSmall
	} else if s.seenCall {
		disp = NoSplit
	}
	odisp, ok := s.funcs[fname]
	if ok {
		s.collisions += 1
		fname = fmt.Sprintf("%s%s%d", fname, "%", s.collisions)
		if odisp > disp {
			disp = odisp
		}
	}
	s.funcs[fname] = disp
	s.seenLong, s.seenShort, s.seenCall = false, false, false
}

func (s *astate) analyze() (leaves int64, nonsplit int64, shortsplit int64, longsplit int64) {
	lv := int64(0)
	ns := int64(0)
	short := int64(0)
	long := int64(0)
	for fn, disp := range s.funcs {
		switch disp {
		case Unknown:
			log.Fatalf("corrupted funcs table entry at %s", fn)
		case Leaf:
			lv += 1
		case NoSplit:
			ns += 1
		case SplitSmall:
			short += 1
		case SplitLarge:
			long += 1
		}
	}
	leaves = lv
	nonsplit = ns
	shortsplit = short
	longsplit = long
	return
}

func examineFile(filename string, detail bool) bool {

	verb(1, "loading ELF for %s", filename)
	_, eerr := elf.Open(filename)
	if eerr != nil {
		warn("%s does not appear to be an ELF file -- ignoring", filename)
		return false
	}

	args := []string{"-d", "--section=.text", "--no-show-raw-insn", filename}
	cmd := exec.Command("objdump", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	verb(1, "cmd started: objdump %v", args)

	var state astate
	state.funcs = make(map[string]FnType)
	curfunc := ""
	fnstart := regexp.MustCompile(`^\S+\s\<(\S+)\>\:\s*$`)
	anycallre := regexp.MustCompile(`^\s*\S+:\s+callq(.+)$`)
	dircallre := regexp.MustCompile(`^\s*\S+\:\s+callq\s+\S+\s+\<(\S+)\>\s*$`)
	pltjumpre := regexp.MustCompile(`^\s*\S+\:\s+jmpq\s+\S+\s+\<\S+@plt\>\s*$`)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		verb(3, "line is %s", line)

		matched := fnstart.FindStringSubmatch(line)
		if matched != nil {
			// Start of new function. Record info for old function.
			if curfunc != "" {
				state.recordFunc(curfunc)
			}
			curfunc = matched[1]
			verb(2, "starting function %s", curfunc)
		}

		pltjumpmatch := pltjumpre.FindStringSubmatch(line)
		if pltjumpmatch != nil {
			state.seenCall = true
		}

		dircallmatch := dircallre.FindStringSubmatch(line)
		if dircallmatch != nil {
			tgt := dircallmatch[1]
			verb(2, ".. direct call to %s", tgt)
			state.seenCall = true
			if tgt == "__morestack" {
				state.seenShort = true
			} else if tgt == "__morestack_non_split" {
				state.seenLong = true
			}
		} else {
			anycallmatch := anycallre.FindStringSubmatch(line)
			if anycallmatch != nil {
				verb(2, ".. anycall to %s", anycallmatch[1])
				state.seenCall = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}

	// Final function
	if curfunc != "" {
		state.recordFunc(curfunc)
	}

	// Post-process
	leaves, nonsplit, shortsplit, longsplit := state.analyze()

	// Emit stats
	fmt.Printf("stats for '%s':\n", filename)
	fmt.Printf("+ leaf functions: %d\n", leaves)
	fmt.Printf("+ nonsplit functions: %d\n", nonsplit)
	fmt.Printf("+ morestack functions: %d\n", shortsplit)
	fmt.Printf("+ morestack_non_split functions: %d\n", longsplit)

	// Now detail
	if detail {
		typs := []FnType{Leaf, NoSplit, SplitSmall, SplitLarge}
		cats := []string{"Leaf", "NoSplit", "MoreStack", "MoreStackNonSplit"}
		for idx := 0; idx < len(typs); idx++ {
			fns := []string{}
			for fn, disp := range state.funcs {
				if disp == typs[idx] {
					fns = append(fns, fn)
				}
			}
			sort.Strings(fns)
			fmt.Printf("\n'%s' functions by name:\n", cats[idx])
			for _, fn := range fns {
				fmt.Printf("%s\n", fn)
			}
		}
	}

	return true
}
