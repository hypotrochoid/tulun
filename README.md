# tulun

Tulun is a tool for generating Chinese language word lists for spaced-repetition system (such as Pleco) export. It takes a list of known words as input, as well as a sequence of target lists which the user wants to learn. The words in a given target list will be presented to the used in natural-language-frequency-order, and additional words will be injected to guarantee that any new character or character component which is introduced will have some number of additional words which use the same component at a nearby position in the final list. The words in earlier specified lists will be presented to the user before words of later specified lists, unless a word in a later list is naturally introduced in an earlier list as a means of increasing usage of an earlier-covered character component.

To Build:
`go build`

Usage:
`tulun --vocab target_list1 --vocab target_list2 --vocab target_list2 --known already_known_words`

All word lists are assumed to be formatted as Pleco flashcards.