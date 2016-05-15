package main

func stringMapKeys(inp map[string]interface{}) []string {
	outp := []string{}
	for k, _ := range inp {
		outp = append(outp, k)
	}
	return outp
}