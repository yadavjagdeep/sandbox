package models

type Post struct {
	URL   string `json:"url"`
	Likes int    `json:"likes"`
}

type HastagMessage struct {
	UserID      int    `json:"user_id"`
	Hashtag     string `json:"hastag"`
	PhotoUrl    string `json:"photo_url"`
	Top100Posts []Post `json:"top_100_posts"`
}

type HastagData struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Top100Posts []Post `json:"top_100_posts"`
	LastUpdate  int64  `json:"last_update"`
}
