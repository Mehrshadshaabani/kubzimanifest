package billing

// MonthlyCheckLimit is how many authenticated /v1/lint calls a plan allows
// per calendar month; 0 means unlimited. Anonymous (unauthenticated) checks
// are never counted here — they're only subject to the API's per-IP rate
// limit. These numbers aren't from any published pricing doc; adjust freely.
var MonthlyCheckLimit = map[Plan]int{
	PlanFree: 50,
	PlanTeam: 1000,
	PlanPro:  0,
}
