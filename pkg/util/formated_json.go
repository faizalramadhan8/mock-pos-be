package util

import "encoding/json"

func FormattedJson(req any) string {
	jsonBody, _ := json.Marshal(req)

	var data interface{}
	if err := json.Unmarshal([]byte(jsonBody), &data); err != nil {
		return "Error formmated json"
	}

	formattedJSON, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "Error formmated json"
	}

	return string(formattedJSON)
}
