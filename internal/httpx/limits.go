package httpx

import "time"

// Rate rules (spec: "Limits and defaults"). Admins are exempt except for login.
var (
	RuleGuestCreate = Rule{"guest.create", 20, time.Hour}
	RulePostCreate  = Rule{"post.create", 5, time.Hour}
	RuleComment     = Rule{"comment.create", 20, time.Hour}
	RuleVote        = Rule{"vote", 120, time.Hour}
	RuleLike        = Rule{"like", 120, time.Hour}
	RuleUpload      = Rule{"upload", 20, time.Hour}
	RulePreview     = Rule{"preview", 120, time.Minute}
	RuleLoginIP     = Rule{"admin.login.ip", 5, 15 * time.Minute}
	RuleLoginUser   = Rule{"admin.login.user", 10, 15 * time.Minute}
)
