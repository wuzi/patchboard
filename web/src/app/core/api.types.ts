// Mirrors internal/api/types.go — the JSON contract with the Go server.
export type I18nName = { pt: string; en: string };
export type Status = { id: number; name: I18nName; color: string; position: number; showOnRoadmap: boolean; isDefault: boolean };
export type Tag = { id: number; name: I18nName; position: number };
export type Board = {
  title: I18nName; description: I18nName; teamName: string; accentColor: string; logoUrl: string | null;
  requireApproval: boolean; uploadsEnabled: boolean; statuses: Status[]; tags: Tag[];
};
export type Author = { kind: 'guest' | 'admin'; id: string; name: string | null };
export type Me = {
  guest: { id: string; name: string | null; recoveryUrl: string } | null;
  admin: { id: number; username: string; displayName: string } | null;
};
export type PostCard = {
  id: number; slug: string; title: string; excerpt: string; statusId: number; tagId: number | null;
  voteCount: number; newVotes: number; commentCount: number; voted: boolean; voters: Author[];
  pinned: boolean; pending: boolean; createdAt: string;
};
export type Image = { id: string; url: string; width: number; height: number };
export type TeamResponse = { html: string; md: string; author: Author; at: string };
export type Comment = {
  id: number; parentId: number | null; author: Author | null; bodyHtml: string | null; bodyMd: string | null;
  likeCount: number; liked: boolean; deleted: boolean; canEdit: boolean; createdAt: string; replies: Comment[];
};
export type PostDetail = PostCard & {
  bodyHtml: string; bodyMd: string; author: Author; images: Image[]; response: TeamResponse | null;
  commentsLocked: boolean; deleted: boolean; canEdit: boolean; canDelete: boolean; comments: Comment[];
};
export type PostRef = { id: number; slug: string };
export type PostResponse = { post: PostDetail | null; mergedInto: PostRef | null };
export type PostPage = { posts: PostCard[]; nextOffset: number | null };
export type Roadmap = { columns: { statusId: number; total: number; posts: PostCard[] }[] };
export type Similar = { posts: { id: number; slug: string; title: string; voteCount: number; statusId: number }[] };
export type UpdateItem = {
  id: number; title: string; bodyHtml: string; bodyMd: string; author: Author; publishedAt: string;
  posts: { id: number; slug: string; title: string; statusId: number }[];
};
export type UpdatePage = { updates: UpdateItem[]; nextOffset: number | null };
export type VoteState = { voteCount: number; voted: boolean };
export type LikeState = { likeCount: number; liked: boolean };
export type TeamMember = { id: number; username: string; displayName: string; createdAt: string; disabled: boolean };
export type Invite = { url: string; expiresAt: string };
export type VoteAudit = {
  voters: { guest: { id: string; name: string | null; createdAt: string }; ipHash: string; votedAt: string; flags: ('shared_ip' | 'new_guest')[] }[];
};
export type Ban = { id: number; guestId: string | null; guestName: string | null; ipHash: string | null; reason: string; createdAt: string };
export type TrashPost = { id: number; slug: string; title: string; deletedAt: string; deletedBy: 'author' | 'admin' };
export type TrashComment = { id: number; postId: number; postTitle: string; excerpt: string; deletedAt: string; deletedBy: 'author' | 'admin' };
export type Trash = { posts: TrashPost[]; comments: TrashComment[] };
export type Settings = {
  title: I18nName; description: I18nName; teamName: string; accentColor: string; logoUploadId: string | null;
  requireApproval: boolean; uploadsEnabled: boolean;
};
export type ChangeStatus = { statusId: number; response?: string; update?: { title: string; body: string } };
export type Sort = 'trending' | 'top' | 'new';
