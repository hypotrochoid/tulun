package main

import (
	"io/ioutil"
	"encoding/json"
	"math"
	"os"
	"strings"
)

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

	if err := json.Unmarshal(input, retv); err != nil {
		panic(err)
	}

	return retv
}

func read_jd(input []byte) map[string]JDLine {
	retv := make(map[string]JDLine)

	if err := json.Unmarshal(input, retv); err != nil {
		panic(err)
	}

	return retv
}

func read_count(input []byte) map[string]int {
	retv := make(map[string]int)

	if err := json.Unmarshal(input, retv); err != nil {
		panic(err)
	}

	return retv
}

func read_json_list(data []byte) map[string][]string {
	retv := make(map[string][]string)

	if err := json.Unmarshal(data, retv); err != nil {
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
		ind :=  strings.Index(line, " ")
		if ind > 0 {
			word := line[:ind]
			remainder := line[(ind+1):]

			w.words = append(w.words, word)
			w.notes = append(w.notes, remainder)
		} else {
			w.words = append(w.words, line)
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
		if line[:2] == "//" {
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

// amount of complexity associated uniquely with a word, independent
// of its roots
// func (s *SRContext) innovation(word string) {}

func (s *SRContext) roots(word string) []string {
	retv := make([]string, 0)
	for char := 0; char < len(word); char+=1 {
		if roots, ok := s.outlier_origins[word[char:(char + 1)]]; ok {
			retv = append(retv, roots...)
		} else if roots, ok := s.heisig_origins[word[char:(char + 1)]]; ok {
			retv = append(retv, roots...)
		}
	}

	return retv
}

// scan though a word to see if it is make of smaller blocks
func (s *SRContext) subwords(word string) []string {
	if len(word) == 1 {
		return []string{word}
	}

	for char := 0; char < len(word); char+=1 {
		// check if there is a dictionary entry for the first char letters
		if _, ok := s.dictionary[word[:(len(word) - char)]]; ok {
			return append([]string{word[:(len(word) - char)]}, s.subwords(word[char:])...)
		}
	}

	return []string{word}
}

func (s *SRContext) decompose_word(word string) []string {
	if len(word) == 1 {
		return s.roots(word)
	} else {
		return s.subwords(word)
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


func main() {
	params := DatafileParams{
		heisig_data:         "data/heisig_decomp.json",
		outlier_data:        "data/outlier_decomp.json",
		stroke_count_data:   "data/char_strokes.json",
		blcu_frequency_data: "data/blcu.json",
		jd_frequency_data:   "data/jd.json",
		dictionary:          "data/cccdict.json",
	}

	ctx := SRContext{}

	ctx.load(params)
	ctx.build_word_graph()


}
