package util

type CharacterMessage struct {
	Character string `json:"character"`
	Text      string `json:"text"`
	Save      bool   `json:"save"`
}

func GetKeys[KeyType comparable, ValueType any](inputMap map[KeyType]ValueType) []KeyType {
	keys := make([]KeyType, 0, len(inputMap))

	for key := range inputMap {
		keys = append(keys, key)
	}

	return keys
}
