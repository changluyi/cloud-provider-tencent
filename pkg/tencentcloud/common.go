package tencentcloud

import "sort"

func isExist(target string, strArray []string) bool {
	index := sort.SearchStrings(strArray, target)
	if index < len(strArray) && strArray[index] == target {
		return true
	}
	return false
}
