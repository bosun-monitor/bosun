package css

// A Parsed stylesheet
type Stylesheet struct {
	Rules []*Rule
}

// Instanciate a new Stylesheet
func NewStylesheet() *Stylesheet {
	return &Stylesheet{}
}

// Returns string representation of the Stylesheet
func (sheet *Stylesheet) String() string {
	result := ""

	for _, rule := range sheet.Rules {
		if result != "" {
			result += "\n"
		}
		result += rule.String()
	}

	return result
}
