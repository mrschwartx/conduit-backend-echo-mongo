package mongodb

import (
	"regexp"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

var duplicateKeyIndex = regexp.MustCompile(`index: (\S+) dup key`)

func IsDuplicateKey(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

func DuplicateKeyIndex(err error) string {
	if !mongo.IsDuplicateKeyError(err) {
		return ""
	}
	match := duplicateKeyIndex.FindStringSubmatch(err.Error())
	if match == nil {
		return ""
	}
	return match[1]
}
