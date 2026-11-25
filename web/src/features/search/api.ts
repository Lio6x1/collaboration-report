import type { SearchResultPage } from '../../../../contracts/http/generated';
import { api } from '../../lib/http';
import type { SearchQuery } from './rules';
import { searchPath } from './rules';

// 使用现有same-origin Cookie传输；浏览器不自报匿名、session或scope证明。
export const readSearchPage = (query: SearchQuery, cursor: string | null) =>
  api<SearchResultPage>(searchPath(query, cursor));
