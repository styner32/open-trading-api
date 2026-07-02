package pulse

import "time"

// kstLocation은 KST 타임존 (Asia/Seoul).
var kstLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()
