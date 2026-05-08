package service

import (
    "regexp"
    "strings"
)

func isQuotaError(err error) bool {
    if err == nil {
        return false
    }
    message := strings.ToLower(err.Error())
    return strings.Contains(message, "quota exceeded") ||
        strings.Contains(message, "429") ||
        regexp.MustCompile(`limit:\s*0`).MatchString(message)
}
