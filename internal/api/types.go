// Package api holds the JSON contract shared by handlers, tests and the Angular client
// (web/src/app/core/api.types.ts mirrors these names).
package api

import "time"

type I18nName struct {
	PT string `json:"pt"`
	EN string `json:"en"`
}

type Status struct {
	ID            int64    `json:"id"`
	Name          I18nName `json:"name"`
	Color         string   `json:"color"`
	Position      int64    `json:"position"`
	ShowOnRoadmap bool     `json:"showOnRoadmap"`
	IsDefault     bool     `json:"isDefault"`
}

type Tag struct {
	ID       int64    `json:"id"`
	Name     I18nName `json:"name"`
	Position int64    `json:"position"`
}

type Board struct {
	Title           I18nName `json:"title"`
	Description     I18nName `json:"description"`
	TeamName        string   `json:"teamName"`
	AccentColor     string   `json:"accentColor"`
	LogoURL         *string  `json:"logoUrl"`
	RequireApproval bool     `json:"requireApproval"`
	UploadsEnabled  bool     `json:"uploadsEnabled"`
	Statuses        []Status `json:"statuses"`
	Tags            []Tag    `json:"tags"`
}

type Author struct {
	Kind string  `json:"kind"` // "guest" | "admin"
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type MeGuest struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	RecoveryURL string  `json:"recoveryUrl"`
}

type MeAdmin struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type Me struct {
	Guest *MeGuest `json:"guest"`
	Admin *MeAdmin `json:"admin"`
}

type PostCard struct {
	ID           int64     `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Excerpt      string    `json:"excerpt"`
	StatusID     int64     `json:"statusId"`
	TagID        *int64    `json:"tagId"`
	VoteCount    int64     `json:"voteCount"`
	NewVotes     int64     `json:"newVotes"`
	CommentCount int64     `json:"commentCount"`
	Voted        bool      `json:"voted"`
	Voters       []Author  `json:"voters"`
	Pinned       bool      `json:"pinned"`
	Pending      bool      `json:"pending"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Image struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int64  `json:"width"`
	Height int64  `json:"height"`
}

type TeamResponse struct {
	HTML   string    `json:"html"`
	MD     string    `json:"md"`
	Author Author    `json:"author"`
	At     time.Time `json:"at"`
}

type PostDetail struct {
	PostCard
	BodyHTML       string        `json:"bodyHtml"`
	BodyMD         string        `json:"bodyMd"`
	Author         Author        `json:"author"`
	Images         []Image       `json:"images"`
	Response       *TeamResponse `json:"response"`
	CommentsLocked bool          `json:"commentsLocked"`
	Deleted        bool          `json:"deleted"`
	CanEdit        bool          `json:"canEdit"`
	CanDelete      bool          `json:"canDelete"`
	Comments       []Comment     `json:"comments"`
}

type PostRef struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
}

type PostResponse struct {
	Post       *PostDetail `json:"post"`
	MergedInto *PostRef    `json:"mergedInto"`
}

type Comment struct {
	ID        int64     `json:"id"`
	ParentID  *int64    `json:"parentId"`
	Author    *Author   `json:"author"`
	BodyHTML  *string   `json:"bodyHtml"`
	BodyMD    *string   `json:"bodyMd"`
	LikeCount int64     `json:"likeCount"`
	Liked     bool      `json:"liked"`
	Deleted   bool      `json:"deleted"`
	CanEdit   bool      `json:"canEdit"`
	CreatedAt time.Time `json:"createdAt"`
	Replies   []Comment `json:"replies"`
}

type PostPage struct {
	Posts      []PostCard `json:"posts"`
	NextOffset *int       `json:"nextOffset"`
}

type RoadmapColumn struct {
	StatusID int64      `json:"statusId"`
	Total    int64      `json:"total"`
	Posts    []PostCard `json:"posts"`
}

type Roadmap struct {
	Columns []RoadmapColumn `json:"columns"`
}

type SimilarPost struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	VoteCount int64  `json:"voteCount"`
	StatusID  int64  `json:"statusId"`
}

type Similar struct {
	Posts []SimilarPost `json:"posts"`
}

type UpdatePost struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	StatusID int64  `json:"statusId"`
}

type UpdateItem struct {
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	BodyHTML    string       `json:"bodyHtml"`
	BodyMD      string       `json:"bodyMd"`
	Author      Author       `json:"author"`
	PublishedAt time.Time    `json:"publishedAt"`
	Posts       []UpdatePost `json:"posts"`
}

type UpdatePage struct {
	Updates    []UpdateItem `json:"updates"`
	NextOffset *int         `json:"nextOffset"`
}

type VoteState struct {
	VoteCount int64 `json:"voteCount"`
	Voted     bool  `json:"voted"`
}

type LikeState struct {
	LikeCount int64 `json:"likeCount"`
	Liked     bool  `json:"liked"`
}

type TeamMember struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	Disabled    bool      `json:"disabled"`
}

type Invite struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type AuditGuest struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuditVoter struct {
	Guest   AuditGuest `json:"guest"`
	IPHash  string     `json:"ipHash"`
	VotedAt time.Time  `json:"votedAt"`
	Flags   []string   `json:"flags"`
}

type VoteAudit struct {
	Voters []AuditVoter `json:"voters"`
}

type Ban struct {
	ID        int64     `json:"id"`
	GuestID   *string   `json:"guestId"`
	GuestName *string   `json:"guestName"`
	IPHash    *string   `json:"ipHash"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

type TrashPost struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	DeletedAt time.Time `json:"deletedAt"`
	DeletedBy string    `json:"deletedBy"`
}

type TrashComment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"postId"`
	PostTitle string    `json:"postTitle"`
	Excerpt   string    `json:"excerpt"`
	DeletedAt time.Time `json:"deletedAt"`
	DeletedBy string    `json:"deletedBy"`
}

type Trash struct {
	Posts    []TrashPost    `json:"posts"`
	Comments []TrashComment `json:"comments"`
}

// UpdateDraft is the Update published together with a status change.
type UpdateDraft struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// ChangeStatusRequest is the body of POST /api/admin/posts/{id}/status.
// Response nil keeps the current team response, "" clears it.
type ChangeStatusRequest struct {
	StatusID int64        `json:"statusId"`
	Response *string      `json:"response"`
	Update   *UpdateDraft `json:"update"`
}
