package tokenizer

func Tokenize(line string) []string {
	var tokens []string
	var current []byte

	for i := 0; i < len(line); i++ {
		c := line[i]

		if c >= 'A' && c <= 'Z' {
			c += 32
		}

		if isLetterOrDigit(c) {
			current = append(current, c)
		} else {
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	return tokens

}

func isLetterOrDigit(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
