import { ActivatedRouteSnapshot, BaseRouteReuseStrategy } from '@angular/router';

/**
 * List, Roadmap, Updates and /p/:idSlug are four routes rendering the same BoardShell.
 * Reusing it keeps the board (and its scroll) behind the post modal instead of re-creating it.
 */
export class BoardReuseStrategy extends BaseRouteReuseStrategy {
  override shouldReuseRoute(future: ActivatedRouteSnapshot, curr: ActivatedRouteSnapshot): boolean {
    if (future.data['shell'] === 'board' && curr.data['shell'] === 'board') return true;
    return super.shouldReuseRoute(future, curr);
  }
}
