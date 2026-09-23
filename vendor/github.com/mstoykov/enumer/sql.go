package main

// Arguments to format are:
//	[1]: type name
const valueMethod = `func (i %[1]s) Value() (driver.Value, error) {
	return i.String(), nil
}
`

const scanMethod = `func (i *%[1]s) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("value is not a byte slice")
		}

		str = string(bytes[:])
	}

	val, err := %[1]sString(str)
	if err != nil {
		return err
	}
	
	*i = val
	return nil
}
`

func (g *Generator) addValueAndScanMethod(typeName string) {
	g.Printf("\n")
	g.Printf(valueMethod, typeName)
	g.Printf("\n\n")
	g.Printf(scanMethod, typeName)
}
