package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "strings"
    "unicode"
)

type Alternation []Symbol

type Symbol struct {
    val string
    isTerminal, isLambda, isNonTerminal, isEOF bool
}

type Rule struct {
    lhs string
    rhs map[int]Alternation
    isStart bool
}

type ProgressRule struct {
    ruleIdx, alternationIdx, pos int
    isFresh, isComplete bool
}

func (p *ProgressRule) equals(other ProgressRule) bool {
    if (p.ruleIdx == other.ruleIdx &&
        p.alternationIdx == other.alternationIdx &&
        p.pos == other.pos &&
        p.isFresh == other.isFresh &&
        p.isComplete == other.isComplete) {
        return true
    }
    return false
}

type ItemSet struct {
    rules []ProgressRule
}

func (self *ItemSet) appendRule(toAdd ProgressRule) {
    for _, rule := range self.rules {
        if rule.equals(toAdd) {
            return
        }
    }
    self.rules = append(self.rules, toAdd)
}

func (self *ItemSet) appendItemSet(toAdd ItemSet) {
    for _, rule := range toAdd.rules {
        self.appendRule(rule)
    }
}

func equals(this, that ItemSet) bool {
    one := this.rules
    two := that.rules
    if len(one) != len(two) {
        return false
    }

    first := make(map[int]bool)
    second := make(map[int]bool)

    for i, firstVal := range one {
        for j, secondVal := range two {
            if !first[i] && !second[j] {
                if firstVal.equals(secondVal) {
                    first[i] = true
                    second[j] = true
                }
            }
        }
    }

    if len(first) != len(second) {
        return false
    }

    if len(first) != len(one) || len(second) != len(two) {
        return false
    }

    return true
}

func main() {
    for _, arg := range os.Args[1:] {
        buildAndOutputCanonicalLRTable(arg)
    }
}

func buildAndOutputCanonicalLRTable(file string) {
    dat, err := ioutil.ReadFile(file)
    if err != nil {
        fmt.Printf("Error reading file - %s\n", file)
    }

    /*
     * 1. parse symbols from data into cfg structs
     * 2. apply "progress positions"
     * 3. run algorithm
     */

    data := string(dat);
    lines := strings.Split(data, "\n")

    rules, err := parseCFG(lines)
    if err != nil {
        fmt.Println("Error parsing CFG")
    }

    startIdx := getStartRuleIndex(rules)
    if startIdx == -1 {
        fmt.Println("Start symbol not found")
    }


    // debug crap
    fmt.Printf("Got %d rules\n", len(rules))
    fmt.Println("CFG:")

    for _, rule := range rules {
        for _, v := range rule.rhs {
            fmt.Printf("%s -> %+v\n", rule.lhs, v)
            /*
            for _, symbol := range v {
                fmt.Printf(" %s", symbol.val)
            }
            fmt.Printf("\n")
            */
        }
    }
    fmt.Println("-------------")

    buildCanonicalLRTable(rules, startIdx)
}

func buildCanonicalLRTable(rules []Rule, startIdx int) {
    symbols := findAllSymbols(rules)
    fmt.Println(symbols)

    freshStart := getFreshStart(rules, startIdx)
    fmt.Println("Fresh Start", freshStart)

    processedState := make(map[int]bool)
    canonicalGoTo := make(map[int]map[string]int)
    states := make(map[int]ItemSet)
    stateIncrementer := 0
    states[stateIncrementer] = closure(rules, freshStart)
    fmt.Println("I(0)", states[0])

    stateIncrementer++

    size := len(states)
    for size == len(states) {
        size = len(states)

        for i, itemSet := range states {
            if !processedState[i] {
                canonicalGoTo[i] = make(map[string]int)

                for _, symbol := range symbols {
                    j := GoTo(rules, itemSet, symbol)
                    if len(j.rules) != 0 && !elementOf(j, states) {
                        canonicalGoTo[i][symbol] = stateIncrementer
                        states[stateIncrementer] = j
                        stateIncrementer++
                    }
                }
            }
        }
    }

    for i := 0; i < len(states); i++ {
        v := states[i]
        fmt.Println("State", i, v)
    }

    for from, transition := range canonicalGoTo {
        for symbol, to := range transition {
            fmt.Printf("Goto(%d, %s): %d\n", from, symbol, to)
        }
    }
}

func elementOf(itemSet ItemSet, set map[int]ItemSet) bool {
    for _, someSet := range set {
        if equals(itemSet, someSet) {
            return true
        }
    }
    return false
}

func parseCFG(lines []string) ([]Rule, error) {
    rules := make([]Rule, 0)

    ruleCount := -1
    for _, line := range lines {
        fields := strings.Fields(line)
        if isNonTerminal(fields[0]) {
            ruleCount++
            rule := parseRule(fields)

            if (fields[0] == "S") {
                rule.isStart = true
            }
            rules = append(rules, rule)
        } else if fields[0] == "|" {
            // parse all the symbols after the |
            symbols := parseSymbols(fields[1:])

            // append onto prior rule
            rule := rules[ruleCount]
            rule.rhs[len(rule.rhs)] = symbols
        } else {
            fmt.Println("Starts with a terminal??", line)
        }
    }

    /*
    s := Symbol{"test", false, false, false, false}
    symbols := Alternation{s}
    list := map[int]Alternation{0:symbols}

    rules[0] = Rule{'S', list}
    */
    return rules, nil
}

func parseRule(fields []string) Rule {
    var rule Rule
    rule.lhs = fields[0]
    // ignore the lhs and "->" field
    symbols := parseSymbols(fields[2:])

    rule.rhs = make(map[int]Alternation)
    rule.rhs[0] = symbols

    rule.isStart = (rule.lhs == "S")
    return rule
}

func parseSymbols(fields []string) Alternation {
    symbols := make(Alternation, 0)
    for _, field := range fields {
        symbol := Symbol{field, false, false, false, false}
        symbol.val = field

        symbol.isNonTerminal = isNonTerminal(field)
        symbol.isTerminal = isTerminal(field)
        symbol.isEOF = isEOF(field)
        symbol.isLambda = isLambda(field)

        symbols = append(symbols, symbol)
    }
    return symbols
}

func findAllSymbols(rules []Rule) []string {
    symbolMap := make(map[string]bool)
    symbols := make([]string, 0)

    for _, rule := range rules {
        symbolMap[rule.lhs] = true

        for _, alternation := range rule.rhs {
            for _, symbol := range alternation {
                if !symbolMap[symbol.val] {
                    symbolMap[symbol.val] = true
                    symbols = append(symbols, symbol.val)
                }
            }
        }
    }

    return symbols
}

func isNonTerminal(val string) bool {
    for _, c := range val {
        if !unicode.IsLetter(c) || !unicode.IsUpper(c) {
            return false
        }
    }
    return true
}

func isTerminal(val string) bool {
    for _, c := range val {
        if !unicode.IsLetter(c) || !unicode.IsLower(c) {
            return false
        }
    }
    return true   
}

func isEOF(val string) bool {
    return (val == "$")
}

func isLambda(val string) bool {
    return (val == "lambda")
}

func getStartRuleIndex(rules []Rule) int {
    startIdx := -1
    for i, rule := range rules {
        if rule.isStart == true {
            return i
        }
    }
    return startIdx
}

func getFreshStart(rules []Rule, idx int) ItemSet {
    var freshStarts ItemSet
    rule := rules[idx]

    for alternationIdx, alternation := range rule.rhs {
        var newProgressRule ProgressRule
        newProgressRule.ruleIdx = idx
        newProgressRule.alternationIdx = alternationIdx
        newProgressRule.pos = 0

        if len(alternation) == 1 && alternation[0].isLambda {
            newProgressRule.isComplete = true
            newProgressRule.isFresh = false
        } else {
            newProgressRule.isComplete = false
            newProgressRule.isFresh = true
        }
        freshStarts.appendRule(newProgressRule)
    }
    return freshStarts
}

func closure(rules []Rule, itemSet ItemSet) ItemSet {
    items := itemSet.rules
    for _, item := range items {
        if !item.isComplete {
            rule := rules[item.ruleIdx]
            alternation := rule.rhs[item.alternationIdx]

            // make sure we can look one ahead into the rule
            if item.pos < len(alternation) {
                symbol := alternation[item.pos]
                if symbol.isNonTerminal {
                    symbolIdx := getNonTerminalRuleIdx(rules, symbol.val)
                    freshStart := getFreshStart(rules, symbolIdx)
                    itemSet.appendItemSet(freshStart)
                }
            } else {
                item.isComplete = true
            }
        }
    }

    return itemSet
}

func GoTo(rules []Rule, itemSet ItemSet, lookFor string) ItemSet {
    var newItemSet ItemSet
    newItemSet.rules = make([]ProgressRule, 0)

    items := itemSet.rules

    // find those with symbol right next to the progress meter
    for _, item := range items {
        if !item.isComplete {
            rule := rules[item.ruleIdx]
            alternation := rule.rhs[item.alternationIdx]

            if item.pos < len(alternation) {
                symbol := alternation[item.pos]

                if !symbol.isLambda && symbol.val == lookFor {
                    item.pos++

                    item.isFresh = false
                    if (item.pos == len(alternation)) {
                        item.isComplete = true
                    }

                    newItemSet.appendRule(item)
                }
            }
        }
    }
    return closure(rules, newItemSet)
}

func getNonTerminalRuleIdx(rules []Rule, nonTerminal string) int {
    for i, rule := range rules {
        if rule.lhs == nonTerminal {
            return i
        }
    }
    return -1
}