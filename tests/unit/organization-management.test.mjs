import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import { parse, compileScript } from '@vue/compiler-sfc';
import ts from 'typescript';

const require = createRequire(import.meta.url);
function moduleURL(source) {
  return (
    'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
      }).outputText,
    ).toString('base64')
  );
}
async function organizationPanel() {
  // 编译真实组件的setup，沿用Vue响应性与HTTP适配；网络边界可控，无需启动产品实例。
  const vue = pathToFileURL(require.resolve('vue/dist/vue.runtime.esm-bundler.js')).href;
  const lifecycle = moduleURL(
    `export * from '${vue}'; export const onMounted = () => {}; export const onUnmounted = () => {};`,
  );
  const read = (path) => readFileSync(new URL('../../web/src/' + path, import.meta.url), 'utf8');
  const http =
    moduleURL(read('lib/http.ts').replace("from 'vue'", `from '${vue}'`)) + '#' + Math.random();
  const rules = moduleURL(read('features/search/rules.ts'));
  const identity = moduleURL(read('features/account/identity.ts'));
  const pagination = moduleURL(read('lib/pagination.ts'));
  const { descriptor } = parse(read('features/organization/OrganizationPanel.vue'));
  const source = compileScript(descriptor, { id: 'organization-management' })
    .content.replaceAll("from 'vue'", `from '${lifecycle}'`)
    .replace("from '../../lib/http'", `from '${http}'`)
    .replace("from '../../lib/pagination'", `from '${pagination}'`)
    .replace("import { user } from '../account/session';", "const user = { value: { id: '101' } };")
    .replace("from '../account/identity'", `from '${identity}'`)
    .replace("from '../search/rules'", `from '${rules}'`)
    .replace(/import (\w+) from '[^']+\.vue';/g, 'const $1 = {};');
  globalThis.location = { pathname: '/organizations', search: '' };
  const component = (await import(moduleURL(source))).default;
  return {
    ...component.setup({}, { expose() {}, emit() {} }),
    transport: await import(http),
  };
}
const response = (value) =>
  new Response(JSON.stringify(value), {
    headers: { 'content-type': 'application/json' },
  });
const page = (items, next_cursor = null) => ({ items, next_cursor, has_more: !!next_cursor });
const membership = (role = 'ADMIN', state = 'ACTIVE') => ({
  user_id: '101',
  role,
  state,
  revision: '17',
});
const organization = (id = '1', current_member = membership()) => ({
  id: String(id),
  owner_id: '500',
  name: '组织' + id,
  lifecycle: 'ACTIVE',
  revision: '1',
  member_revision: '2',
  current_member,
});
const member = (id, role = 'MEMBER') => ({
  user_id: String(id),
  role,
  state: 'ACTIVE',
  display_name: '成员' + id,
  revision: '1',
  knowledge_base_creation: role === 'ADMIN' ? 'ROLE' : 'NONE',
});
const application = (id) => ({
  id: String(id),
  organization_id: '1',
  applicant_id: '201',
  reason: '加入组织',
  state: 'PENDING',
  revision: '1',
});
const invitation = (id) => ({
  id: String(id),
  organization_id: '1',
  state: 'OPEN',
  expires_at: '2026-10-14T00:00:00Z',
  revision: '1',
});
function deferred() {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
const failureResponse = (status) =>
  new Response(
    JSON.stringify({ code: status === 503 ? 'SERVICE_UNAVAILABLE' : 'ORGANIZATION_ACCESS_DENIED' }),
    { status, headers: { 'content-type': 'application/json' } },
  );
function managementResponse(path) {
  const pathname = new URL(path, 'https://test.invalid').pathname;
  if (pathname === '/v1/organizations') return response(page([organization()]));
  if (pathname === '/v1/account/applications') return response(page([]));
  const match = pathname.match(/^\/v1\/organizations\/(1|2)(?:\/(members|applications|invitations))?$/);
  assert.ok(match, '测试应明确处理每个HTTP边界');
  const [, id, directory] = match;
  const owner = id === '1' ? '500' : '600';
  if (!directory) return response({ ...organization(id), owner_id: owner });
  const items =
    directory === 'members'
      ? [member(owner)]
      : directory === 'applications'
        ? [{ ...application('approval' + id), organization_id: id }]
        : [{ ...invitation('invitation' + id), organization_id: id }];
  return response(page(items, 'second-page'));
}

// 本人不在当前页也须打开管理区；权限读取不得为找本人预取后页。
test('management uses detail membership and reads only one member page', async () => {
  const panel = await organizationPanel();
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    if (path === '/v1/organizations/1') return response(organization());
    if (path.includes('/members'))
      return response(
        page(
          Array.from({ length: 20 }, (_, i) => member(i + 1)),
          'page2',
        ),
      );
    return response(
      page(path.includes('/applications') ? [application('approval')] : [invitation('invitation')]),
    );
  };
  await panel.openOrg('1');
  await panel.loadManagement();
  assert.equal(panel.admin.value, true);
  assert.equal(panel.mine.value.revision, '17');
  assert.equal(panel.managementLoaded.value, true);
  assert.equal(panel.applications.value[0]?.id, 'approval');
  assert.equal(panel.invites.value[0]?.id, 'invitation');
  assert.equal(requests.filter((path) => path.includes('/members')).length, 1);
  assert.equal(panel.memberPage.next, 'page2', '其余成员仅在用户翻页时读取');
});

// 可见行可能来自旧成员事实，不能反过来提升详情中当前成员的管理资格。
for (const [label, current_member] of [
  ['ordinary member', membership('MEMBER')],
  ['revoking administrator', membership('ADMIN', 'REVOKING')],
  ['missing membership', undefined],
]) {
  test(`member rows cannot grant management to a ${label}`, async () => {
    const panel = await organizationPanel();
    const requests = [];
    globalThis.fetch = async (path) => {
      requests.push(path);
      if (path === '/v1/organizations/1') return response({ ...organization(), current_member });
      return response(page([member('101', 'ADMIN')], 'repeated'));
    };
    await panel.openOrg('1');
    await panel.loadManagement();
    assert.equal(panel.admin.value, false);
    assert.equal(panel.managementLoaded.value, true);
    assert.equal(requests.filter((path) => path.includes('/members')).length, 1);
    assert.equal(
      requests.some((path) => /\/(applications|invitations)\?/.test(path)),
      false,
      '未经当前详情确认的角色不能读取管理列表',
    );
  });
}

// 切换组织后晚到的目录不能把旧管理员身份、成员或管理列表装到新组织下。
test('switching organizations while reading management discards the old page', async () => {
  const panel = await organizationPanel();
  const firstPage = deferred();
  const reading = deferred();
  const requests = [];
  globalThis.fetch = async (path) => {
    requests.push(path);
    if (path === '/v1/organizations/1') return response(organization());
    if (path === '/v1/organizations/2') return response(organization('2', membership('MEMBER')));
    if (path.includes('/members')) {
      reading.resolve();
      return firstPage.promise;
    }
    return response(page([]));
  };
  await panel.openOrg('1');
  const loading = panel.loadManagement();
  await reading.promise;
  await panel.openOrg('2');
  firstPage.resolve(response(page([member('101', 'ADMIN')], 'old-next')));
  await loading;
  assert.equal(panel.selected.value.id, '2');
  assert.equal(panel.admin.value, false);
  assert.equal(panel.members.value.length, 0);
  assert.equal(panel.memberPage.next, null);
  assert.equal(panel.managementLoaded.value, false);
  assert.equal(
    requests.some((path) => /\/(applications|invitations)\?/.test(path)),
    false,
  );
});

test('leaving uses current detail membership revision when the member is not visible', async () => {
  const panel = await organizationPanel();
  let leaveBody;
  globalThis.fetch = async (path, options) => {
    if (path === '/v1/organizations/1') return response(organization('1', membership('MEMBER')));
    if (path.endsWith('/leave')) {
      leaveBody = JSON.parse(options.body);
      return response({ operation_id: leaveBody.operation_id, state: 'SUCCEEDED' });
    }
    return response(page(path.includes('/members') ? [member('1')] : []));
  };
  await panel.openOrg('1');
  await panel.loadManagement();
  await panel.leave();
  assert.equal(leaveBody?.expected_revision, '17');
  assert.equal(panel.selected.value, null);
  assert.equal(panel.error.value, '');
});

const lists = [
  {
    label: 'organizations',
    path: '/v1/organizations',
    load: 'listOrg',
    rows: 'organizations',
    state: 'orgPage',
    item: organization,
    key: 'id',
  },
  {
    label: 'members',
    path: '/v1/organizations/1/members',
    load: 'loadMembers',
    rows: 'members',
    state: 'memberPage',
    item: member,
    key: 'user_id',
  },
  {
    label: 'approvals',
    path: '/v1/organizations/1/applications',
    load: 'loadApps',
    rows: 'applications',
    state: 'appPage',
    item: application,
    key: 'id',
  },
  {
    label: 'invitations',
    path: '/v1/organizations/1/invitations',
    load: 'loadInvites',
    rows: 'invites',
    state: 'invitePage',
    item: invitation,
    key: 'id',
  },
  {
    label: 'own applications',
    path: '/v1/account/applications',
    load: 'ownApps',
    rows: 'ownApplications',
    state: 'ownPage',
    item: application,
    key: 'id',
  },
];

for (const list of lists) {
  // 如果翻页恢复为append、取错游标或上一页读内存，这个真实组件行为测试会失败。
  test(`${list.label} replaces twenty-row pages and rereads previous pages from the server`, async () => {
    const panel = await organizationPanel();
    const requests = [];
    let firstReads = 0;
    globalThis.fetch = async (path) => {
      if (path === '/v1/organizations/1') return response({ ...organization(), owner_id: '101' });
      const url = new URL(path, 'https://test.invalid');
      assert.equal(url.pathname, list.path);
      requests.push(url);
      if (url.searchParams.has('cursor')) return response(page([list.item('21')]));
      firstReads++;
      return response(
        page(
          Array.from({ length: 20 }, (_, i) => list.item(String(firstReads * 100 + i))),
          'second-page',
        ),
      );
    };
    await panel.openOrg('1');
    await panel[list.load]();
    await panel[list.load](1);
    assert.deepEqual(
      panel[list.rows].value.map((row) => row[list.key]),
      ['21'],
    );
    assert.equal(panel[list.state].index, 1);
    assert.equal(requests[1].searchParams.get('cursor'), 'second-page');
    assert.equal(requests[1].searchParams.get('limit'), '20');
    await panel[list.load](0);
    assert.equal(panel[list.rows].value.length, 20);
    assert.equal(panel[list.rows].value[0][list.key], '200');
    assert.equal(panel[list.state].index, 0);
    assert.equal(requests[2].searchParams.has('cursor'), false);
    assert.equal(requests[2].searchParams.get('limit'), '20');
  });

  // 同组织、同筛选也可能发生先发后到，组织代次本身不足以保护这些读取。
  test(`${list.label} ignores a superseded response within the same scope`, async () => {
    const panel = await organizationPanel();
    const late = deferred();
    const reading = deferred();
    let reads = 0;
    globalThis.fetch = async (path) => {
      if (path === '/v1/organizations/1') return response({ ...organization(), owner_id: '101' });
      const url = new URL(path, 'https://test.invalid');
      assert.equal(url.pathname, list.path);
      if (++reads === 1) {
        reading.resolve();
        return late.promise;
      }
      return response(page([list.item('22')], 'fresh-next'));
    };
    await panel.openOrg('1');
    const oldRequest = panel[list.load]();
    await reading.promise;
    await panel[list.load]();
    late.resolve(response(page([list.item('11')], 'old-next')));
    await oldRequest;
    assert.deepEqual(
      panel[list.rows].value.map((row) => row[list.key]),
      ['22'],
    );
    assert.equal(panel[list.state].next, 'fresh-next');
  });

  test(`${list.label} cannot install a response from a previous session generation`, async () => {
    const panel = await organizationPanel();
    const late = deferred();
    const reading = deferred();
    globalThis.fetch = async (path) => {
      if (path === '/v1/organizations/1') return response(organization());
      assert.equal(new URL(path, 'https://test.invalid').pathname, list.path);
      reading.resolve();
      return late.promise;
    };
    await panel.openOrg('1');
    const oldRequest = panel[list.load]();
    await reading.promise;
    panel.transport.rotateSessionContext();
    late.resolve(response(page([list.item('11')], 'old-next')));
    await assert.rejects(oldRequest, (error) => error.code === 'SESSION_CHANGED');
    assert.deepEqual(panel[list.rows].value, []);
    assert.equal(panel[list.state].next, null);
  });
}

test('member search starts a server-filtered first page and discards an older unfiltered page', async () => {
  const panel = await organizationPanel();
  const late = deferred();
  const reading = deferred();
  const requests = [];
  globalThis.fetch = async (path) => {
    if (path === '/v1/organizations/1') return response(organization());
    const url = new URL(path, 'https://test.invalid');
    requests.push(url);
    if (url.searchParams.has('q')) return response(page([member('77')]));
    if (url.searchParams.has('cursor')) {
      reading.resolve();
      return late.promise;
    }
    return response(page([member('1')], 'second-page'));
  };
  await panel.openOrg('1');
  await panel.loadMembers();
  const oldPage = panel.loadMembers(1);
  await reading.promise;
  panel.memberSearch.value = '  搜索&成员  ';
  await panel.searchMembers();
  late.resolve(response(page([member('21')], 'third-page')));
  await oldPage;
  assert.deepEqual(
    panel.members.value.map((row) => row.user_id),
    ['77'],
  );
  assert.equal(panel.memberPage.index, 0);
  assert.equal(panel.memberPage.next, null);
  assert.deepEqual(panel.memberPage.cursors, [null]);
  assert.equal(requests.at(-1).searchParams.get('q'), '搜索&成员');
  assert.equal(requests.at(-1).searchParams.get('limit'), '20');
  assert.equal(requests.at(-1).searchParams.has('cursor'), false);
  assert.equal(panel.admin.value, true, '搜索隐藏本人时管理入口仍来自当前详情');

  panel.memberSearch.value = '00000101';
  await panel.searchMembers();
  assert.equal(requests.at(-1).searchParams.get('q'), '00000101', '八位账号直接交由服务端匹配');
});

test('an ordinary member retains a known owner name across pages and search until the organization changes', async () => {
  const panel = await organizationPanel();
  globalThis.fetch = async (path) => {
    if (path === '/v1/organizations/1') return response(organization('1', membership('MEMBER')));
    if (path === '/v1/organizations/2')
      return response({ ...organization('2', membership('MEMBER')), owner_id: '600' });
    const url = new URL(path, 'https://test.invalid');
    if (url.searchParams.has('q')) return response(page([member('22')]));
    if (url.searchParams.has('cursor')) return response(page([member('21')]));
    return response(page([{ ...member('500'), display_name: '已读取的所有者' }], 'second-page'));
  };
  await panel.openOrg('1');
  await panel.loadMembers();
  assert.equal(panel.ownerName.value, '已读取的所有者');
  await panel.loadMembers(1);
  assert.equal(panel.ownerName.value, '已读取的所有者');
  panel.memberSearch.value = '成员22';
  await panel.searchMembers();
  assert.equal(panel.ownerName.value, '已读取的所有者');
  await panel.openOrg('2');
  assert.equal(panel.ownerName.value, '');
  assert.equal(panel.memberSearch.value, '');
  assert.equal(panel.memberPage.index, 0);
});

test('a failed member search can refresh the current organization from its unfiltered first page', async () => {
  const panel = await organizationPanel();
  const memberRequests = [];
  globalThis.fetch = async (path) => {
    if (path === '/v1/organizations/1') return response(organization('1', membership('MEMBER')));
    const url = new URL(path, 'https://test.invalid');
    if (url.pathname.endsWith('/members')) {
      memberRequests.push(url);
      if (url.searchParams.has('q'))
        return new Response(JSON.stringify({ code: 'SERVICE_UNAVAILABLE' }), {
          status: 503,
          headers: { 'content-type': 'application/json' },
        });
      return response(
        url.searchParams.has('cursor') ? page([member('21')]) : page([member('1')], 'second-page'),
      );
    }
    return response(page([]));
  };
  panel.detailPane.value = 'management';
  await panel.openOrg('1');
  await panel.loadMembers(1);
  panel.memberSearch.value = '失败查询';
  await panel.searchMembers();
  assert.notEqual(panel.error.value, '');
  assert.deepEqual(panel.members.value, []);
  assert.equal(panel.memberPage.index, 0);
  assert.equal(panel.memberPage.next, null);
  await panel.action.run(panel.refresh, '');
  assert.equal(panel.error.value, '');
  assert.equal(panel.memberQuery.value, '');
  assert.equal(panel.memberPage.index, 0);
  assert.deepEqual(
    panel.members.value.map((row) => row.user_id),
    ['1'],
  );
  assert.equal(memberRequests.at(-1).searchParams.has('q'), false);
  assert.equal(memberRequests.at(-1).searchParams.has('cursor'), false);
});

const organizationDirectories = lists.filter((list) =>
  ['loadMembers', 'loadApps', 'loadInvites'].includes(list.load),
);
for (const list of organizationDirectories) {
  for (const status of [401, 403, 404, 410]) {
    // 授权失败不能只显示错误，仍把首页的敏感目录与旧管理员入口留在页面中。
    test(`${list.label} clears the visible page and organization capabilities after a current ${status}`, async () => {
      const panel = await organizationPanel();
      const late = deferred();
      const reading = deferred();
      globalThis.fetch = async (path) => {
        if (path.startsWith(list.path + '?') && path.includes('cursor=')) {
          reading.resolve();
          return late.promise;
        }
        return managementResponse(path);
      };
      await panel.openOrg('1');
      await panel.loadManagement();
      assert.equal(panel.admin.value, true);
      assert.equal(panel.ownerName.value, '成员500');
      const loading = panel.action.run(() => panel[list.load](1), '');
      await reading.promise;
      const visibleDuringLoad = [...panel[list.rows].value];
      late.resolve(failureResponse(status));
      await loading;
      assert.deepEqual(visibleDuringLoad, [], '请求开始就卸下旧页，不让首页冒充下一页');
      assert.deepEqual(panel.members.value, []);
      assert.deepEqual(panel.applications.value, []);
      assert.deepEqual(panel.invites.value, []);
      assert.equal(panel.memberPage.next, null);
      assert.equal(panel.appPage.next, null);
      assert.equal(panel.invitePage.next, null);
      assert.equal(panel.selected.value, null);
      assert.equal(panel.mine.value, undefined);
      assert.equal(panel.admin.value, false);
      assert.equal(panel.owner.value, false);
      assert.equal(panel.ownerName.value, '');
      assert.equal(panel.managementLoaded.value, false);
      assert.notEqual(panel.error.value, '');
      await panel.action.run(panel.refresh, '');
      assert.equal(panel.organizations.value[0]?.id, '1', '刷新仍可重新发现有权组织');
      assert.equal(panel.error.value, '');
    });
  }

  test(`${list.label} leaves its old rows empty on 503 while retaining retryable current membership`, async () => {
    const panel = await organizationPanel();
    globalThis.fetch = async (path) =>
      path.startsWith(list.path + '?') && path.includes('cursor=')
        ? failureResponse(503)
        : managementResponse(path);
    await panel.openOrg('1');
    await panel.loadManagement();
    await panel.action.run(() => panel[list.load](1), '');
    assert.deepEqual(panel[list.rows].value, [], '503也不能让上一页冒充本次请求成功');
    assert.equal(panel.selected.value?.id, '1');
    assert.equal(panel.mine.value?.revision, '17');
    assert.equal(panel.admin.value, true, '暂时不可用不证明当前成员已撤权');
    assert.equal(panel[list.state].index, 0);
    assert.equal(panel[list.state].next, 'second-page', '原入口游标可供重试');
    assert.notEqual(panel.error.value, '');
    globalThis.fetch = async (path) => managementResponse(path);
    await panel.action.run(() => panel[list.load](1), '');
    assert.equal(panel[list.state].index, 1);
    assert.notEqual(panel[list.rows].value.length, 0);
    assert.equal(panel.error.value, '');
  });

  test(`${list.label} cannot clear a new organization after an old 403 arrives`, async () => {
    const panel = await organizationPanel();
    const late = deferred();
    const reading = deferred();
    globalThis.fetch = async (path) => {
      if (path.startsWith(list.path + '?') && path.includes('cursor=')) {
        reading.resolve();
        return late.promise;
      }
      return managementResponse(path);
    };
    await panel.openOrg('1');
    await panel.loadManagement();
    const oldRequest = panel[list.load](1);
    await reading.promise;
    await panel.openOrg('2');
    await panel.loadManagement();
    late.resolve(failureResponse(403));
    await assert.rejects(oldRequest, (error) => error.status === 403);
    assert.equal(panel.selected.value?.id, '2');
    assert.equal(panel.admin.value, true);
    assert.equal(panel.ownerName.value, '成员600');
    assert.equal(panel.members.value[0]?.user_id, '600');
    assert.equal(panel.applications.value[0]?.id, 'approval2');
    assert.equal(panel.invites.value[0]?.id, 'invitation2');
  });

  test(`${list.label} cannot revoke current state from an old session failure`, async () => {
    const panel = await organizationPanel();
    const late = deferred();
    const reading = deferred();
    globalThis.fetch = async (path) => {
      if (path.startsWith(list.path + '?') && path.includes('cursor=')) {
        reading.resolve();
        return late.promise;
      }
      return managementResponse(path);
    };
    await panel.openOrg('1');
    await panel.loadManagement();
    const oldRequest = panel[list.load](1);
    await reading.promise;
    panel.transport.rotateSessionContext();
    late.resolve(failureResponse(401));
    await assert.rejects(oldRequest, (error) => error.code === 'SESSION_CHANGED');
    assert.equal(panel.selected.value?.id, '1');
    assert.equal(panel.admin.value, true);
    assert.equal(panel.ownerName.value, '成员500');
    assert.deepEqual(panel[list.rows].value, []);
  });

  test(`${list.label} cannot clear a later successful request after a superseded 404`, async () => {
    const panel = await organizationPanel();
    const late = deferred();
    const reading = deferred();
    globalThis.fetch = async (path) => {
      if (path.startsWith(list.path + '?') && path.includes('cursor=')) {
        reading.resolve();
        return late.promise;
      }
      return managementResponse(path);
    };
    await panel.openOrg('1');
    await panel.loadManagement();
    const oldRequest = panel[list.load](1);
    await reading.promise;
    await panel[list.load](0);
    late.resolve(failureResponse(404));
    await assert.rejects(oldRequest, (error) => error.status === 404);
    assert.equal(panel.selected.value?.id, '1');
    assert.equal(panel.admin.value, true);
    assert.notEqual(panel[list.rows].value.length, 0);
  });
}

test('an authorization failure prevents another pending management response from restoring revoked rows', async () => {
  const panel = await organizationPanel();
  const late = deferred();
  const reading = deferred();
  globalThis.fetch = async (path) => {
    if (path.includes('/invitations?') && path.includes('cursor=')) {
      reading.resolve();
      return late.promise;
    }
    if (path.includes('/members?') && path.includes('cursor=')) return failureResponse(403);
    return managementResponse(path);
  };
  await panel.openOrg('1');
  await panel.loadManagement();
  const oldInvitations = panel.loadInvites(1);
  await reading.promise;
  await assert.rejects(panel.loadMembers(1), (error) => error.status === 403);
  late.resolve(response(page([invitation('late-invitation')])));
  await oldInvitations;
  assert.equal(panel.selected.value, null);
  assert.equal(panel.admin.value, false);
  assert.deepEqual(panel.invites.value, []);
});

test('clearing revoked organization reads preserves an unknown write and retries its original operation', async () => {
  const panel = await organizationPanel();
  const posted = [];
  let recoverWrite = false;
  globalThis.fetch = async (path, options) => {
    if (path === '/v1/organizations' && options.method === 'POST') {
      const body = JSON.parse(options.body);
      posted.push(body);
      if (!recoverWrite) throw new TypeError('connection lost');
      return response({ operation_id: body.operation_id, state: 'SUCCEEDED' });
    }
    if (path.includes('/members?') && path.includes('cursor=')) return failureResponse(404);
    return managementResponse(path);
  };
  await panel.openOrg('1');
  await panel.loadManagement();
  panel.name.value = '原未知操作';
  await panel.createOrg();
  const original = panel.pending.value;
  const retry = panel.retry.value;
  assert.notEqual(original, '');
  await assert.rejects(panel.loadMembers(1), (error) => error.status === 404);
  assert.equal(panel.selected.value, null);
  assert.equal(panel.pending.value, original);
  assert.equal(panel.retry.value, retry);
  recoverWrite = true;
  await retry();
  assert.equal(posted.length, 2);
  assert.equal(posted[0].operation_id, original);
  assert.equal(posted[1].operation_id, original);
  assert.equal(posted[1].name, '原未知操作');
  assert.equal(panel.pending.value, '');
});
