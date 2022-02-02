package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

type ustring []rune

func rslice(str string, begin, end int) string {
	if end >= 0 {
		return string(ustring(str)[begin:end])
	}

	return string(ustring(str)[begin:])
}

func rlen(str string) int {
	return utf8.RuneCountInString(str)
}

type MemoryParams struct {
	char_innovation float64
	basal_decay float64
	rep_hardening float64
	max_distraction float64
}

func (m *MemoryParams) remember_prob(delay float64, reps float64) float64 {
	return math.Exp(m.basal_decay*delay*(1.0/(m.rep_hardening * reps + 1.0)))
}

type SRContext struct {
	outlier_origins map[string][]string
	heisig_origins map[string][]string

	blcu_words_freq map[string]int
	jd_words map[string]JDLine

	char_strokes map[string]int

	dictionary map[string]CDictLine

	word_tree map[string]WordParams
}

// DatafileParams gives the files used for word analysis
type DatafileParams struct {
	heisig_data string
	outlier_data string
	stroke_count_data string
	blcu_frequency_data string
	jd_frequency_data string
	dictionary string
}

type WordParams struct {
	word string
	depth int
	frequency int
	strokes int
	direct_parents []string
	direct_children []string
	definition string
}

type CDictLine struct {
	t string // traditional
	p string // pronunciation
	d string // definition
}


type JDLine struct {
	f string // frequency
	p string // pronunciation
	d string // definition
}

func read_cedict(input []byte) map[string]CDictLine {
	retv := make(map[string]CDictLine)

	if err := json.Unmarshal(input, &retv); err != nil {
		if jsonErr, ok := err.(*json.SyntaxError); ok {
     	   problemPart := input[jsonErr.Offset-10 : jsonErr.Offset+10]
        	err = fmt.Errorf("%w ~ error near '%s' (offset %d)", err, problemPart, jsonErr.Offset)
    	}
		panic(err)
	}

	return retv
}

func read_jd(input []byte) map[string]JDLine {
	retv := make(map[string]JDLine)

	if err := json.Unmarshal(input, &retv); err != nil {
		if jsonErr, ok := err.(*json.SyntaxError); ok {
     	   problemPart := input[jsonErr.Offset-10 : jsonErr.Offset+10]
        	err = fmt.Errorf("%w ~ error near '%s' (offset %d)", err, problemPart, jsonErr.Offset)
    	}
		panic(err)
	}

	return retv
}

func read_count(input []byte) map[string]int {
	retv := make(map[string]int)

	if err := json.Unmarshal(input, &retv); err != nil {
		if jsonErr, ok := err.(*json.SyntaxError); ok {
     	   problemPart := input[jsonErr.Offset-10 : jsonErr.Offset+10]
        	err = fmt.Errorf("%w ~ error near '%s' (offset %d)", err, problemPart, jsonErr.Offset)
    	}
		panic(err)
	}

	return retv
}

func read_json_list(data []byte) map[string][]string {
	retv := make(map[string][]string)

	if err := json.Unmarshal(data, &retv); err != nil {
		panic(err)
	}

	return retv
}

type WordListStage struct {
	name string
	words []string
	notes []string
}

func (w *WordListStage) load(lines []string) {
	// first line is the name
	w.name = lines[0]
	lines = lines[1:]

	for _, line := range lines {
		ind :=  strings.Index(line, "\t")
		if ind > 0 {
			word := line[:ind]
			remainder := line[(ind+1):]

			w.words = append(w.words, string(word))
			w.notes = append(w.notes, remainder)
		} else if(len(line) > 0){
			w.words = append(w.words, string(line))
			w.notes = append(w.notes, "")
		}

	}
}

type WordList struct {
	stages []WordListStage
}

func (w *WordList) load(input string) {
	lines := strings.Split(input, "\n")
	new_phase := []string{}

	phases := [][]string{}

	for _, line := range lines {
		if (rlen(line) > 1) && (line[:2] == "//") {
			// new phase
			if len(new_phase) > 0 {
				phases = append(phases, new_phase)
			}
			new_phase = []string{line}
		} else {

			new_phase = append(new_phase, line)
		}
	}

	phases = append(phases, new_phase)

	for _, phase := range phases {
		new_stage := WordListStage{}
		new_stage.load(phase)
		w.stages = append(w.stages, new_stage)
	}
}


// the degree to which knowing source_word strengthens target_word
//
// in truth this should be measured empirically, but we'll do what we can
func (s *SRContext) explanatory_similarity(source_word, target_word string) float64 {

	return 0
}

func load_file(name string) []byte {
	file, err := os.Open(name)
	if err != nil {
		panic(err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	return data
}

func (s *SRContext) load(files DatafileParams) {
	s.outlier_origins = read_json_list(load_file(files.outlier_data))
	s.heisig_origins = read_json_list(load_file(files.heisig_data))
	s.blcu_words_freq = read_count(load_file(files.blcu_frequency_data))
	s.char_strokes	= read_count(load_file(files.stroke_count_data))
	s.jd_words = read_jd(load_file(files.jd_frequency_data))
	s.dictionary = read_cedict(load_file(files.dictionary))
}

func (s *SRContext) roots(word string) []string {
	if roots, ok := s.outlier_origins[word]; ok {
		return roots
	} 
	// TODO: removing because it causes cycles, make more robust
	//else if roots, ok := s.heisig_origins[word]; ok {
	//	return roots
	//}

	return []string{}
}

// scan though a word to see if it is make of smaller blocks
func (s *SRContext) subwords(word string) []string {
	if rlen(word) == 1 {
		return []string{word}
	}

	w1 := rslice(word, 0, 1)
	w2 := rslice(word, 1, -1)

	for breakpt := 1; breakpt < rlen(word); breakpt++ {
		// check if there is a dictionary entry for the first char letters
		w1p := rslice(word, 0, breakpt)
		w2p := rslice(word, breakpt, -1)
		
		if _, ok := s.dictionary[w1]; ok {
			w1 = w1p
			w2 = w2p
		}
	}
	
	return append([]string{w1}, s.subwords(w2)...)
}

func filter_str(words []string, remove string) []string {
	// sorry
	rv := []string{}

	for _, wd := range words {
		if wd != remove{
			rv = append(rv, wd)
		}
	}

	return rv
}

func (s *SRContext) decompose_word(word string) []string {
	if rlen(word) == 1 {
		// need to filter as a consequence of cyclic dependency of using both heisig and outlier
		return filter_str(s.roots(word), word)
	} else {
		return filter_str(s.subwords(word), word)
	}
}

func append_child(orig WordParams, word string) WordParams {
	return WordParams{
		word: orig.word,
		depth: orig.depth,
		frequency: orig.frequency,
		strokes: orig.strokes,
		direct_parents: orig.direct_parents,
		direct_children: append(orig.direct_children, word),
		definition: orig.definition,
	}
}

// assumes that all basic data files are already loaded
func (s *SRContext) build_word_graph() {
	remaining_words := make(map[string]WordParams)
	undefined_roots := make(map[string]WordParams)

	s.word_tree = make(map[string]WordParams)

	for word := range s.dictionary {
		strokes := 0
		for _, char := range word {
			strokes += s.char_strokes[string(char)]
		}

		wp := WordParams{
			word: word,
			frequency: s.blcu_words_freq[word],
			strokes: strokes,
			direct_parents: s.decompose_word(word),
			definition: s.dictionary[word].d,
		}

		remaining_words[word] = wp

		// backfill any undefined roots
		for _, word := range wp.direct_parents {
			if _, ok := s.dictionary[word]; !ok {
				// this word isn't in the dictionary
				if _, ok := undefined_roots[word]; !ok {
					undefined_roots[word] = WordParams{
						word: word,
						frequency: s.blcu_words_freq[word],
						strokes: s.char_strokes[word],
					}
				}
			}
		}
	}

	for word, wp := range undefined_roots {
		remaining_words[word] = wp
	}

	// fill in children
	for word, wp := range remaining_words {
		if _, ok := s.word_tree[word]; !ok {
			s.word_tree[word] = wp
		}

		for _, parent := range wp.direct_parents {
			if _, ok := s.word_tree[parent]; !ok {
				s.word_tree[parent] = remaining_words[parent]
			}

			s.word_tree[parent] = append_child(s.word_tree[parent], word)
		}
	}

	// fill in word depth
	for word, wp := range s.word_tree {
		depth := s.compute_dependency_depth(word)
		wp.depth = depth
		s.word_tree[word] = wp
	}
}

// how many layers deep the word can go
func (s *SRContext) compute_dependency_depth(word string) int {
	wp := s.word_tree[word]
	if( len(wp.direct_parents) == 0 ){
		return 0
	}

	max_depth := 0
	for _, parent := range wp.direct_parents {
		pdepth := s.compute_dependency_depth(parent)
		if (pdepth + 1) > max_depth {
			max_depth = pdepth + 1
		}
	}

	return max_depth
}

func (s *SRContext) frequency(word string) int {
	return s.blcu_words_freq[word]
}

func (s *SRContext) frequency_sort(words []string) []string {
	sort.Slice(words, 
		func(i, j int) bool { 
			return s.frequency(words[i]) > s.frequency(words[j])
		})

	return words
}

func (s *SRContext) parent_sequence(known map[string]bool, target string, expansion int) []string {
	parents := s.word_tree[target].direct_parents
	retv := []string{}
	
	known[target] = true

	for _, parent := range parents{
		if !known[parent] {			
			siblings := s.word_tree[parent].direct_children
			unused_siblings := []string{}
			known_siblings := []string{}
			for _, sibling := range siblings {
				if sibling == target {
					continue
				}
				if !known[sibling] {
					// maybe use it
					unused_siblings = append(unused_siblings, sibling)
				} else {
					// fill in gaps
					known_siblings = append(known_siblings, sibling)
				}
			}
			unused_sorted := s.frequency_sort(unused_siblings)

			target_siblings := append(known_siblings, unused_sorted...)

			nx := expansion
			if len(target_siblings) < expansion {
				nx = len(target_siblings)
			}
		
			// prevent infinite loops
			known[parent] = true
			retv = append(retv, parent)

			for i := 0; i < nx; i++ {
				known[target_siblings[i]] = true

				sibling_parents := s.parent_sequence(known, target_siblings[i], expansion)
				for _, sp := range sibling_parents {
					if !known[sp]{
						retv = append(retv, sp)
					}
				}

				retv = append(retv, target_siblings[i])
			}
		}
		// maybe do something for known parents?

	}


	retv = append(retv, target)

	return retv
}

func (s *SRContext) compute_sequence(known_list []string, target_list []string) []string {
	common_unit_factor := 3
	card_sequence := []string{}


	presence := make(map[string]bool)
	for _, word := range known_list {
		presence[word] = true
	}

	target_list = s.frequency_sort(target_list)
	for _, target := range target_list {
		parent_seq := s.parent_sequence(presence, target, common_unit_factor)
		card_sequence = append(card_sequence, parent_seq...)
	}

	return card_sequence
}


// Created so that multiple inputs can be accepted.
type arrayFlags []string

func (i *arrayFlags) String() string {
	retv := ""
	for _, str := range *i {
		retv += str + " "
	}

	return retv
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
}


func main() {
	params := DatafileParams{
		heisig_data:         "data/heisig_decomp.json",
		outlier_data:        "data/outlier_decomp.json",
		stroke_count_data:   "data/char_strokes.json",
		blcu_frequency_data: "data/blcu.json",
		jd_frequency_data:   "data/jd.json",
		dictionary:          "data/cccedict.json",
	}

	ctx := SRContext{}

	ctx.load(params)
	ctx.build_word_graph()

	targets := arrayFlags{}

	flag.Var(&targets, "vocab", "list of desired vocab words")
	known_file := flag.String("known", "", "list of already known words")

	flag.Parse()

	known_list := WordList{}
	known_list.load(string(load_file(*known_file)))

	known := []string{}
	for _, chars := range known_list.stages {
		known = append(known, chars.words...)
	}

	target_lists := WordList{}
	for _, tlist := range targets {
		target_lists.load(string(load_file(tlist)))
	}

	final_list := []string{}
	for _, target := range target_lists.stages {
		seq := ctx.compute_sequence(known, target.words)
		final_list = append(final_list, seq...)
		known = append(known, seq...)
	}

	for _, card := range final_list {
		fmt.Println(card)
	}
}
