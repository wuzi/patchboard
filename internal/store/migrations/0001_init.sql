CREATE TABLE board_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  title_pt TEXT NOT NULL, title_en TEXT NOT NULL,
  description_pt TEXT NOT NULL DEFAULT '', description_en TEXT NOT NULL DEFAULT '',
  team_name TEXT NOT NULL,
  accent_color TEXT NOT NULL,
  logo_upload_id TEXT REFERENCES uploads(id) ON DELETE SET NULL,
  require_approval INTEGER NOT NULL DEFAULT 0,
  uploads_enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE statuses (
  id INTEGER PRIMARY KEY,
  name_pt TEXT NOT NULL, name_en TEXT NOT NULL,
  color TEXT NOT NULL, position INTEGER NOT NULL,
  show_on_roadmap INTEGER NOT NULL DEFAULT 1,
  is_default INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX statuses_one_default ON statuses(is_default) WHERE is_default = 1;
CREATE TABLE tags (id INTEGER PRIMARY KEY, name_pt TEXT NOT NULL, name_en TEXT NOT NULL, position INTEGER NOT NULL);
CREATE TABLE guests (
  id TEXT PRIMARY KEY,                 -- public id: 16 chars lowercase base32
  token_hash BLOB NOT NULL UNIQUE,     -- sha256(cookie token)
  name TEXT,
  created_at INTEGER NOT NULL,
  created_ip_hash TEXT NOT NULL,
  last_ip_hash TEXT NOT NULL
);
CREATE TABLE admins (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  disabled_at INTEGER
);
CREATE TABLE admin_sessions (
  token_hash BLOB PRIMARY KEY, admin_id INTEGER NOT NULL REFERENCES admins(id),
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL
);
CREATE TABLE admin_invites (
  token_hash BLOB PRIMARY KEY, created_by INTEGER NOT NULL REFERENCES admins(id),
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL,
  used_at INTEGER, used_by INTEGER REFERENCES admins(id)
);
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL, title TEXT NOT NULL,
  body_md TEXT NOT NULL DEFAULT '', body_html TEXT NOT NULL DEFAULT '', excerpt TEXT NOT NULL DEFAULT '',
  status_id INTEGER NOT NULL REFERENCES statuses(id),
  tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL,
  author_guest_id TEXT REFERENCES guests(id),
  author_admin_id INTEGER REFERENCES admins(id),
  ip_hash TEXT NOT NULL,
  vote_count INTEGER NOT NULL DEFAULT 0,
  comment_count INTEGER NOT NULL DEFAULT 0,
  pending INTEGER NOT NULL DEFAULT 0,
  pinned_at INTEGER,
  comments_locked INTEGER NOT NULL DEFAULT 0,
  merged_into_id INTEGER REFERENCES posts(id),
  deleted_at INTEGER, deleted_by_admin_id INTEGER REFERENCES admins(id),
  response_md TEXT, response_html TEXT, response_admin_id INTEGER REFERENCES admins(id), response_at INTEGER,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  CHECK ((author_guest_id IS NULL) <> (author_admin_id IS NULL))
);
CREATE INDEX posts_visible ON posts(status_id, vote_count) WHERE deleted_at IS NULL AND merged_into_id IS NULL AND pending = 0;
CREATE TABLE votes (
  post_id INTEGER NOT NULL REFERENCES posts(id), guest_id TEXT NOT NULL REFERENCES guests(id),
  ip_hash TEXT NOT NULL, created_at INTEGER NOT NULL,
  PRIMARY KEY (post_id, guest_id)
);
CREATE INDEX votes_recent ON votes(post_id, created_at);
CREATE TABLE comments (
  id INTEGER PRIMARY KEY,
  post_id INTEGER NOT NULL REFERENCES posts(id),
  parent_id INTEGER REFERENCES comments(id),
  author_guest_id TEXT REFERENCES guests(id), author_admin_id INTEGER REFERENCES admins(id),
  body_md TEXT NOT NULL, body_html TEXT NOT NULL,
  ip_hash TEXT NOT NULL,
  like_count INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  deleted_at INTEGER, deleted_by_admin_id INTEGER REFERENCES admins(id),
  CHECK ((author_guest_id IS NULL) <> (author_admin_id IS NULL))
);
CREATE INDEX comments_post ON comments(post_id, created_at);
CREATE TABLE comment_likes (
  comment_id INTEGER NOT NULL REFERENCES comments(id), guest_id TEXT NOT NULL REFERENCES guests(id),
  created_at INTEGER NOT NULL, PRIMARY KEY (comment_id, guest_id)
);
CREATE TABLE uploads (
  id TEXT PRIMARY KEY,                 -- 20 chars lowercase base32
  ext TEXT NOT NULL CHECK (ext IN ('jpg','png')),
  width INTEGER NOT NULL, height INTEGER NOT NULL, bytes INTEGER NOT NULL,
  post_id INTEGER REFERENCES posts(id), position INTEGER NOT NULL DEFAULT 0,
  guest_id TEXT REFERENCES guests(id), admin_id INTEGER REFERENCES admins(id),
  ip_hash TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE updates (
  id INTEGER PRIMARY KEY, title TEXT NOT NULL, body_md TEXT NOT NULL, body_html TEXT NOT NULL,
  author_admin_id INTEGER NOT NULL REFERENCES admins(id),
  published_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE update_posts (
  update_id INTEGER NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
  post_id INTEGER NOT NULL REFERENCES posts(id),
  status_id INTEGER NOT NULL REFERENCES statuses(id),   -- snapshot at link time
  position INTEGER NOT NULL,
  PRIMARY KEY (update_id, post_id)
);
CREATE TABLE bans (
  id INTEGER PRIMARY KEY, guest_id TEXT REFERENCES guests(id), ip_hash TEXT,
  reason TEXT NOT NULL DEFAULT '', created_by INTEGER NOT NULL REFERENCES admins(id), created_at INTEGER NOT NULL,
  CHECK (guest_id IS NOT NULL OR ip_hash IS NOT NULL)
);
CREATE INDEX bans_guest ON bans(guest_id);
CREATE INDEX bans_ip ON bans(ip_hash);
CREATE VIRTUAL TABLE posts_fts USING fts5(title, body_md, content='posts', content_rowid='id',
  tokenize='unicode61 remove_diacritics 2');
CREATE TRIGGER posts_fts_ai AFTER INSERT ON posts BEGIN
  INSERT INTO posts_fts(rowid, title, body_md) VALUES (new.id, new.title, new.body_md); END;
CREATE TRIGGER posts_fts_ad AFTER DELETE ON posts BEGIN
  INSERT INTO posts_fts(posts_fts, rowid, title, body_md) VALUES ('delete', old.id, old.title, old.body_md); END;
CREATE TRIGGER posts_fts_au AFTER UPDATE OF title, body_md ON posts BEGIN
  INSERT INTO posts_fts(posts_fts, rowid, title, body_md) VALUES ('delete', old.id, old.title, old.body_md);
  INSERT INTO posts_fts(rowid, title, body_md) VALUES (new.id, new.title, new.body_md); END;
