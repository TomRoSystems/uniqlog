package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arbovm/levenshtein"
)

// when two lines reach this value we start to track them back
var FIRST_SIMILARITY_THRESHOLD = 0.9

// what is the level of other lines that has to match
var KEEP_SIMILARITY_THRESHOLD = 0.49

// debug mode
var DEBUG = 0

// if pattern is repeating many times in a long file print number of occurencies so far
const TIME_WITHOUT_OUTPUT = 2*time.Second

// how many lines we track back (circular buffer size)
const MAX_LINES_TRACK = 50


const ColorWhite = "\033[1;38m"
const ColorRed = "\033[1;31m"
const ColorGreen = "\033[1;32m"
const ColorMagenta = "\033[1;35m"
const ColorYellow = "\033[1;33m"

const ColorBlue = "\033[1;34m"
const ColorGray = "\033[1;90m"

const ColorBgDark = "\033[1;100m"

const ColorReset = "\033[0m"

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// this regular expressions are helper for finding similarities
// TODO: add later for IP addresses or other things in logs that differs but means same thing
var REG_HEX_VALUE = regexp.MustCompile("^[0-9a-f]+$")
var REG_HTTP_METHOD = regexp.MustCompile("^GET|HEAD|PUT|POST$")

// Read more: How git similarity index is calculated?
func GetSimilarity(prevTokens, tokens []string) float64 {
	sameTokens := 0
	totalTokens := len(tokens)
	if totalTokens < 1 {
		return 0
	}

	// here we are assuming that first one is a timestamp
	if totalTokens < 3 { // we are comparing here only one token
		idx := totalTokens - 1 // index of token to compare

		if prevTokens[idx] == tokens[idx] {
			return 1
		}
		// special cases
		if REG_HEX_VALUE.MatchString(prevTokens[idx]) && REG_HEX_VALUE.MatchString(prevTokens[idx]) {
			if len(prevTokens[idx]) == len(tokens[idx]) {
				return 0.9
			}
		}
		if REG_HTTP_METHOD.MatchString(prevTokens[idx]) && REG_HTTP_METHOD.MatchString(prevTokens[idx]) {
			return 0.9
		}

		dist := levenshtein.Distance(prevTokens[idx], tokens[idx])
		return 1 - (float64(dist) / float64(max(len(prevTokens[idx]), len(tokens[idx]))))
	}

	compareTokens := totalTokens - 1
	for i := 0; i < compareTokens; i++ {
		if i >= len(prevTokens) {
			break
		}
		if tokens[totalTokens-1-i] == prevTokens[len(prevTokens)-1-i] {
			sameTokens++
		}
	}
	return float64(sameTokens) / float64(compareTokens)
}

func PrintSimilarity(prvLine, curLine string) string {
	res := ""

	for idx, _ := range prvLine {
		cmp := byte(0)
		if idx < len(curLine) {
			cmp = curLine[idx]
		}
		if prvLine[idx] == cmp {
			res = res + ColorWhite + string(prvLine[idx])
		} else {
			res = res + ColorReset + string(prvLine[idx])
		}
	}

	return res + ColorReset
}

// this function is to call our loop one more time beyond scanner.Scan()
func CallOneMoreTime(lastLine *bool) bool {
	if *lastLine == false {
		*lastLine = true
		return true
	}
	return false
}

func Perform(input io.Reader) {
	ll := make([]string, MAX_LINES_TRACK) // last lines
	li := 0                               // last index (for circular-buffer)

	simLines := make([]string, 0) // number of lines in a similarity block
	simBlockIndex := 0
	simBlockRepeats := 0

	out := 0 // number of flushed lines
	inp := 0 // number of read lines

	lastLine := false
	line := ""

	lastOutputTime := time.Now()

	scanner := bufio.NewScanner(input)
	for scanner.Scan() || CallOneMoreTime(&lastLine) {
		if lastLine {
			line = ""
		} else {
			line = scanner.Text()
			if len(line) < 2 {
				continue
			}
		}

		inp++
		tokens := strings.Fields(line)

		simBlockLines := len(simLines)

		n := time.Now()

		if simBlockLines > 0 { // we are checking does similiary perseve
			simBlockIndex = (simBlockIndex + 1) % simBlockLines

			similarity := GetSimilarity(strings.Fields(simLines[simBlockIndex]), tokens)
			if similarity > KEEP_SIMILARITY_THRESHOLD {
				if simBlockIndex+1 >= simBlockLines {
					simBlockRepeats++
					if time.Since(lastOutputTime) >= TIME_WITHOUT_OUTPUT { // display something after a while
						fmt.Printf("%s... work in progres (%v repeats)..%s\n", ColorGray, simBlockRepeats, ColorReset)
						lastOutputTime = n
					}
				}
			} else { // not anymore
				if simBlockRepeats > 0 {
					prefix := ColorBgDark // ColorWhite
					if simBlockLines == 1 {
						prefix = ""
						simBlockRepeats++
					}
					// print fragment that was repeating
					for i := 0; i < simBlockLines; i++ {
						fmt.Printf("%s%s\n", prefix, simLines[i])
						out++
					}
					fmt.Printf(ColorReset+"\t\t\t\t"+ColorWhite+"-- repeated %d more times --"+ColorReset+"\n", simBlockRepeats)
					out += simBlockLines * simBlockRepeats
				} else { // there was not even one full repetition
					if (DEBUG > 0) {
						fmt.Printf("%s--- Failed at %+v range: %+v sim: %+v%s\n", ColorMagenta, simBlockIndex, simBlockLines, similarity, ColorReset)
					}
					if simBlockIndex == 30 {
						inp++
						inp--
					}
					for i := 0; i < simBlockIndex; i++ {
						idx := (li - simBlockLines + i + simBlockIndex - 1 + MAX_LINES_TRACK) % MAX_LINES_TRACK
						res := PrintSimilarity(simLines[i], ll[idx])
						fmt.Printf("%s\n", res)
						out++
					}
					if (DEBUG > 0) { // here we print those two lines which caused mismatch in pattern
						fmt.Printf("%s-%s%s\n", ColorMagenta, ColorRed, line)
						fmt.Printf(" %s%s\n", ColorRed, simLines[simBlockIndex])
						out++
						// and the remianing ones
						for i := simBlockIndex; i < simBlockLines; i++ {
							fmt.Printf("%s%s\n", ColorReset, simLines[i])
							out++
						}
						fmt.Printf("%s---%s\n", ColorMagenta, ColorReset)
						lastOutputTime = n
					}
				}
				simBlockLines = 0
				simLines = make([]string, 0)
			}

		}

		if simBlockLines == 0 { // looking for similarites
			unscanned := inp - out - 1
			for i := 0; i < MAX_LINES_TRACK; i++ {
				if i >= unscanned {
					break
				}
				idx := (li - i - 1 + MAX_LINES_TRACK) % MAX_LINES_TRACK
				similarity := GetSimilarity(strings.Fields(ll[idx]), tokens)
				if similarity > FIRST_SIMILARITY_THRESHOLD { // was 0.6
					simBlockLines = i + 1
					// print all unscanned ones - those which are in-between
					for j := 0; j < unscanned-simBlockLines; j++ {
						jdx := (li - unscanned + j + MAX_LINES_TRACK) % MAX_LINES_TRACK
						fmt.Printf("%s\n", ll[jdx])
						out++
					}
					// set our "window"
					simBlockRepeats = 0
					simBlockIndex = 0

					// create tokens to compare with
					simLines = make([]string, simBlockLines)
					for j := range simLines {
						jdx := (li - simBlockLines + j + MAX_LINES_TRACK) % MAX_LINES_TRACK
						simLines[j] = ll[jdx]
					}

					break
				}
			}
		}

		// update circular buffer
		if out+MAX_LINES_TRACK <= inp && simBlockLines == 0 { // we cannot hold it longer
			fmt.Printf("%s\n", ll[li]) // so we have to flush it
			lastOutputTime = n
			out++
		}
		ll[li] = line
		li = (li + 1) % MAX_LINES_TRACK

		if lastLine { // flush all remaining lines
			for idx := (li - (inp - out) + MAX_LINES_TRACK) % MAX_LINES_TRACK; out < inp; out++ {
				fmt.Printf("%s\n", ll[idx]) // so we have to flush it
				idx = (idx + 1) % MAX_LINES_TRACK
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
	}
}

func printHelp() {
	fmt.Println("uniqlog find simmilar lines in log files")
	fmt.Printf("Usage: %s [-fst %v] [-kst %v] [<file>]\n", os.Args[0], FIRST_SIMILARITY_THRESHOLD, KEEP_SIMILARITY_THRESHOLD)
	fmt.Println("Options:")
	fmt.Printf("-fst\tfirst similarity threshold (default %v)\n", FIRST_SIMILARITY_THRESHOLD)
	fmt.Printf("-kst\tkeep similarity threshold (default %v)\n", KEEP_SIMILARITY_THRESHOLD)
	fmt.Printf("-d\tdebug mode (print more information when line mismatches)\n")
	fmt.Printf("-help\tprint this help information\n")
}

func main() {
	var inp io.Reader = os.Stdin
	skipStdin := false
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch a {
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		case "-d":
			DEBUG++
		case "-fst":
			i++
			FIRST_SIMILARITY_THRESHOLD, _ = strconv.ParseFloat(os.Args[i], 32)
		case "-kst":
			i++
			KEEP_SIMILARITY_THRESHOLD, _ = strconv.ParseFloat(os.Args[i], 32)
		default:
			skipStdin = true
			file, err := os.Open(a)
			defer file.Close()
			if err != nil {
				log.Fatalf("unable to open file %s - %+v", a, err)
			}
			Perform(file)
		case "-":
			skipStdin = false
			break
		}
	}

	if !skipStdin {
		Perform(inp)
	}
}
