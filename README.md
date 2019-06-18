# analyze-stacksplit

This is a tool (written in Go) that analyzes gccgo/gollvm binary stack-split prolog characteristics. Intended as a development/debugging tool for compiler developers.

Usage:


```
% analyze-stacksplit /tmp/himom.exe
stats for '/tmp/himom.exe:
+ leaf functions: 237
+ nonsplit functions: 172
+ morestack functions: 3891
+ morestack_non_split functions: 441
...
%
```

You can also ask for a detailed report using the "-detail" command line flag. This will dump out the names of the functions in each category.

Notes:

* tested/usable only for linux/amd64 binaries
* currently works by parsing the output of 'objdump -d', which is brittle/fragile (also can be confused by assembly functions, which might not pattern-match properly)




