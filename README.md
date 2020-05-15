# Uniqlog
This is simple tool to find similar repeating lines in log files.

## Installation
Once cloned / downloaded run:

```
go get
go build
```

## Instruction
By default it reads from stdin and uses colors to print similiarties.

Run: `uniqlog test/test01.txt` to see output.

## TODO
* Process log in JSON format
* Add more function(s) to check similiarity for things like IP address, filenames/paths, etc.
* Make color works for windows?

## License
BSD
