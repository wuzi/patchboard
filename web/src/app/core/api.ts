import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import type { Observable } from 'rxjs';
import type {
  Ban, Board, ChangeStatus, Comment, I18nName, Image, Invite, LikeState, Me, PostCard, PostDetail, PostPage,
  PostResponse, Roadmap, Settings, Similar, Sort, Status, Tag, TeamMember, Trash, UpdateItem, UpdatePage, VoteAudit, VoteState,
} from './api.types';

function params(p: Record<string, string | number | undefined | null>): HttpParams {
  let hp = new HttpParams();
  for (const [k, v] of Object.entries(p)) if (v !== undefined && v !== null && v !== '') hp = hp.set(k, String(v));
  return hp;
}

/** Typed client for the PatchBoard JSON API (one method per endpoint). */
@Injectable({ providedIn: 'root' })
export class Api {
  private readonly http = inject(HttpClient);

  // board & identity
  board = (): Observable<Board> => this.http.get<Board>('/api/board');
  me = (): Observable<Me> => this.http.get<Me>('/api/me');
  setName = (name: string): Observable<Me> => this.http.put<Me>('/api/me/name', { name });
  preview = (markdown: string): Observable<{ html: string }> => this.http.post<{ html: string }>('/api/markdown/preview', { markdown });

  // posts
  listPosts = (p: { sort?: Sort; status?: number | null; tag?: number | null; q?: string; offset?: number }): Observable<PostPage> =>
    this.http.get<PostPage>('/api/posts', { params: params(p) });
  roadmap = (p: { tag?: number | null; q?: string }): Observable<Roadmap> => this.http.get<Roadmap>('/api/roadmap', { params: params(p) });
  similar = (title: string): Observable<Similar> => this.http.get<Similar>('/api/posts/similar', { params: params({ title }) });
  post = (id: number): Observable<PostResponse> => this.http.get<PostResponse>(`/api/posts/${id}`);
  createPost = (b: { title: string; body: string; tagId: number | null; imageIds: string[] }): Observable<PostDetail> =>
    this.http.post<PostDetail>('/api/posts', b);
  updatePost = (id: number, b: { title?: string; body?: string; tagId?: number; clearTag?: boolean; imageIds?: string[] }): Observable<PostDetail> =>
    this.http.patch<PostDetail>(`/api/posts/${id}`, b);
  deletePost = (id: number): Observable<void> => this.http.delete<void>(`/api/posts/${id}`);
  vote = (id: number): Observable<VoteState> => this.http.post<VoteState>(`/api/posts/${id}/vote`, null);
  unvote = (id: number): Observable<VoteState> => this.http.delete<VoteState>(`/api/posts/${id}/vote`);

  // comments
  comment = (postId: number, body: string, parentId?: number | null): Observable<Comment> =>
    this.http.post<Comment>(`/api/posts/${postId}/comments`, parentId ? { body, parentId } : { body });
  editComment = (id: number, body: string): Observable<Comment> => this.http.patch<Comment>(`/api/comments/${id}`, { body });
  deleteComment = (id: number): Observable<void> => this.http.delete<void>(`/api/comments/${id}`);
  like = (id: number): Observable<LikeState> => this.http.post<LikeState>(`/api/comments/${id}/like`, null);
  unlike = (id: number): Observable<LikeState> => this.http.delete<LikeState>(`/api/comments/${id}/like`);

  // uploads & updates
  upload = (file: File): Observable<Image> => {
    const fd = new FormData();
    fd.append('file', file);
    return this.http.post<Image>('/api/uploads', fd);
  };
  updates = (offset = 0): Observable<UpdatePage> => this.http.get<UpdatePage>('/api/updates', { params: params({ offset }) });

  // admin: accounts
  adminLogin = (username: string, password: string): Observable<Me> => this.http.post<Me>('/api/admin/login', { username, password });
  adminLogout = (): Observable<void> => this.http.post<void>('/api/admin/logout', null);
  checkInvite = (token: string): Observable<{ valid: boolean }> => this.http.get<{ valid: boolean }>(`/api/admin/invites/${token}`);
  acceptInvite = (token: string, b: { username: string; displayName: string; password: string }): Observable<Me> =>
    this.http.post<Me>(`/api/admin/invites/${token}/accept`, b);
  createInvite = (): Observable<Invite> => this.http.post<Invite>('/api/admin/invites', null);
  team = (): Observable<TeamMember[]> => this.http.get<TeamMember[]>('/api/admin/team');
  disableMember = (id: number): Observable<void> => this.http.post<void>(`/api/admin/team/${id}/disable`, null);
  updateAdminMe = (b: { displayName?: string; currentPassword?: string; newPassword?: string }): Observable<Me> =>
    this.http.patch<Me>('/api/admin/me', b);

  // admin: board
  saveSettings = (s: Settings): Observable<Board> => this.http.put<Board>('/api/admin/settings', s);
  createStatus = (b: { name: I18nName; color: string; showOnRoadmap: boolean }): Observable<Status> => this.http.post<Status>('/api/admin/statuses', b);
  updateStatus = (id: number, b: Partial<{ name: I18nName; color: string; showOnRoadmap: boolean; isDefault: boolean }>): Observable<Status> =>
    this.http.patch<Status>(`/api/admin/statuses/${id}`, b);
  orderStatuses = (ids: number[]): Observable<Status[]> => this.http.put<Status[]>('/api/admin/statuses/order', { ids });
  deleteStatus = (id: number, moveTo?: number): Observable<void> =>
    this.http.delete<void>(`/api/admin/statuses/${id}`, { params: params({ moveTo }) });
  createTag = (name: I18nName): Observable<Tag> => this.http.post<Tag>('/api/admin/tags', { name });
  updateTag = (id: number, name: I18nName): Observable<Tag> => this.http.patch<Tag>(`/api/admin/tags/${id}`, { name });
  orderTags = (ids: number[]): Observable<Tag[]> => this.http.put<Tag[]>('/api/admin/tags/order', { ids });
  deleteTag = (id: number): Observable<void> => this.http.delete<void>(`/api/admin/tags/${id}`);

  // admin: moderation
  changeStatus = (id: number, b: ChangeStatus): Observable<PostDetail> => this.http.post<PostDetail>(`/api/admin/posts/${id}/status`, b);
  pending = (): Observable<PostCard[]> => this.http.get<PostCard[]>('/api/admin/pending');
  approve = (id: number): Observable<PostDetail> => this.http.post<PostDetail>(`/api/admin/posts/${id}/approve`, null);
  pin = (id: number, pinned: boolean): Observable<PostDetail> => this.http.post<PostDetail>(`/api/admin/posts/${id}/pin`, { pinned });
  lock = (id: number, locked: boolean): Observable<PostDetail> => this.http.post<PostDetail>(`/api/admin/posts/${id}/lock`, { locked });
  restorePost = (id: number): Observable<void> => this.http.post<void>(`/api/admin/posts/${id}/restore`, null);
  restoreComment = (id: number): Observable<void> => this.http.post<void>(`/api/admin/comments/${id}/restore`, null);
  merge = (id: number, intoId: number): Observable<PostDetail> => this.http.post<PostDetail>(`/api/admin/posts/${id}/merge`, { intoId });
  trash = (): Observable<Trash> => this.http.get<Trash>('/api/admin/trash');
  voteAudit = (id: number): Observable<VoteAudit> => this.http.get<VoteAudit>(`/api/admin/posts/${id}/votes`);
  removeVote = (id: number, guestId: string): Observable<VoteState> => this.http.delete<VoteState>(`/api/admin/posts/${id}/votes/${guestId}`);
  ban = (b: { guestId: string; includeIp: boolean; reason: string }): Observable<Ban> => this.http.post<Ban>('/api/admin/bans', b);
  bans = (): Observable<Ban[]> => this.http.get<Ban[]>('/api/admin/bans');
  unban = (id: number): Observable<void> => this.http.delete<void>(`/api/admin/bans/${id}`);
  deleteUpload = (id: string): Observable<void> => this.http.delete<void>(`/api/admin/uploads/${id}`);
  createUpdate = (b: { title: string; body: string; postIds: number[] }): Observable<UpdateItem> => this.http.post<UpdateItem>('/api/admin/updates', b);
  editUpdate = (id: number, b: { title?: string; body?: string; postIds?: number[] }): Observable<UpdateItem> =>
    this.http.patch<UpdateItem>(`/api/admin/updates/${id}`, b);
  deleteUpdate = (id: number): Observable<void> => this.http.delete<void>(`/api/admin/updates/${id}`);
}
