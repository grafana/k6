package highlight

// DetectFiletype will use the list of syntax definitions provided and the filename and first line of the file
// to determine the filetype of the file
// It will return the corresponding syntax definition for the filetype
func DetectFiletype(defs []*Def, filename string, firstLine []byte) *Def {
	for _, d := range defs {
		if d.ftdetect[0].MatchString(filename) {
			return d
		}
		if len(d.ftdetect) > 1 {
			if d.ftdetect[1].MatchString(string(firstLine)) {
				return d
			}
		}
	}

	emptyDef := new(Def)
	emptyDef.FileType = "Unknown"
	emptyDef.rules = new(rules)
	return emptyDef
}
