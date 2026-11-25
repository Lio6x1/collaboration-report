"""将人工业务讲解与 AST 中可核对的执行证据放在同一落笔步骤旁。"""
import html
import json
import re
from pathlib import Path


def code(value):
    # HTML code 避免 SQL 反引号、逻辑或、尖括号破坏 Markdown 表格。
    value = str(value)
    suffix = ''
    if len(value) > 220:
        value = value[:180] + ' …'
        suffix = '（长表达式，按本步完整代码继续）'
    return '<code>' + html.escape(value).replace('|', '&#124;').replace('[', '&#91;').replace(']', '&#93;').replace('\n', ' ') + '</code>' + suffix


def load_notes(directory, entries):
    notes = {}
    for path in sorted(Path(directory).glob('*.json')):
        for name, value in json.loads(path.read_text(encoding='utf-8'))['files'].items():
            if name in notes:
                raise ValueError('DUPLICATE_FILE_EXPLANATION: ' + name)
            notes[name] = value
    for entry in entries:
        name = entry['path']
        if name not in notes:
            raise ValueError('FILE_EXPLANATION_MISSING: ' + name)
        note = notes[name]
        for key in ('purpose', 'why', 'flow', 'pitfalls'):
            if not note.get(key):
                raise ValueError('FILE_EXPLANATION_EMPTY: ' + name + ' ' + key)
        if note.get('source_sha256') != entry['sha256']:
            raise ValueError('FILE_EXPLANATION_OUTDATED: ' + name)
    return notes


def validate_symbols(path, note, declarations):
    symbols = {name for declaration in declarations for name in declaration['symbols']}
    symbols.update(name for declaration in declarations for member in declaration.get('members') or []
                   for name in member['symbols'])
    unknown = set(note.get('symbols', {})) - symbols
    if unknown:
        raise ValueError('EXPLANATION_SYMBOL_NOT_FOUND: ' + path + ' :: ' + ', '.join(sorted(unknown)))
    items = [item for declaration in declarations for item in [declaration] + (declaration.get('members') or [])]
    for symbol, detail in note.get('symbols', {}).items():
        matches = [item for item in items if symbol in item['symbols']]
        if any(item['kind'] == 'sql' for item in matches):
            continue
        for category in ('fields', 'params', 'results'):
            available = {str(index) if category == 'results' else field['name']
                         for item in matches for index, field in enumerate(item.get(category) or [])}
            if category == 'params':
                available.update(str(index) for item in matches for index, field in enumerate(item.get(category) or []))
            extra = set(detail.get(category, {})) - available
            if extra:
                raise ValueError('EXPLANATION_FIELD_NOT_FOUND: ' + path + ' :: ' + symbol + ' ' + category + ' ' + ', '.join(sorted(extra)))


def require_complete(path, rendered):
    if '<!-- explanation-gap -->' in rendered:
        raise ValueError('EXPLANATION_GAP: ' + path)


def bind_type_contexts(outline, notes):
    """按实际包目录及import别名定位类型，避免把同名handler/repository/logic串线。"""
    definitions = {}
    for path, declarations in outline.items():
        package = str(Path(path).parent).replace('\\', '/')
        for d in declarations:
            if d['kind'] != 'type':
                continue
            for symbol in d['symbols']:
                definitions.setdefault((package, symbol), []).append((path, d))
    for path, note in notes.items():
        package = str(Path(path).parent).replace('\\', '/')
        packages = {'': package}
        for alias, imported in note.get('_imports', {}).items():
            if imported.startswith('collaboration/'):
                name, _, suffix = imported.removeprefix('collaboration/').partition('/')
                prefix = 'services/' if name in ('account', 'document', 'search') else 'packages/'
                packages[alias + '.'] = prefix + name + ('/' + suffix if suffix else '')
        contexts = {}
        for qualifier, target_package in packages.items():
            for (candidate_package, name), matches in definitions.items():
                if candidate_package != target_package or len(matches) != 1:
                    continue
                target, declaration = matches[0]
                detail = notes[target].get('symbols', {}).get(name, {})
                description = detail.get('why') or declaration.get('comment')
                if not description and target != path:
                    description = notes[target]['purpose']
                if description:
                    contexts[qualifier + name] = '使用 ' + code(qualifier + name) + ' 类型；其定义处的职责：' + description.strip()
        note['_type_contexts'] = contexts


def file_explanation(note):
    text = '## 为什么创建这个文件\n\n' + note['purpose'] + '\n\n' + note['why'] + '\n\n'
    text += '写之前先按这条执行路线理解各段的连接：\n\n'
    text += '\n'.join(f'{i}. {item}' for i, item in enumerate(note['flow'], 1)) + '\n\n'
    text += '本文件需要守住的边界：\n\n' + '\n'.join('- ' + item for item in note['pitfalls']) + '\n\n'
    return text


def symbol_note(note, declaration):
    result = {}
    for name in declaration.get('symbols', []):
        result.update(note.get('symbols', {}).get(name, {}))
    return result


TYPE_MEANINGS = {
    'context.Context': '此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。',
    '*sql.DB': '服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。',
    '*sql.Tx': '调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。',
    '*sql.Conn': '已经取得的单条数据库连接；连接级锁或会话状态依赖同一连接，不能换回连接池执行。',
    'error': '失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。',
    'http.ResponseWriter': '当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。',
    '*http.Request': '当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。',
    'time.Duration': '一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。',
    'time.Time': '一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。',
    'sync.Mutex': '保护共享内存的互斥锁；加锁与解锁必须覆盖同一临界区，不能代替数据库事务或跨进程锁。',
    '*http.Client': '注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。',
    '*platform.Store': '本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。',
    'platform.Config': '经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。',
    '*codec.Validator': '冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。',
    '*pb.OperationContext': '内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。',
    '*grpc.Server': '当前进程内部gRPC服务器，业务装配向它注册冻结接口，底座负责TLS与服务器生命周期。',
    '*grpc.ClientConn': '复用的内部RPC连接，调用沿各自ctx；连接创建者负责关闭，不要每次RPC重建连接。',
    'io.Reader': '按需读取字节的输入流，不保证一次Read就拿到完整内容；调用者负责期限、体积限制和最终完整性核验。',
    'io.Writer': '接收输出字节的流，写入可能失败或短写；不能只看业务计算完成就宣称文件已输出。',
    'json.RawMessage': '尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。',
    'http.HandlerFunc': '处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。',
}

# 跨模块含义稳定的业务词；刻意不包括 id、value、target、key 等需要局部上下文的名字。
TERMS = {
    'OperationID': '一次业务意图的幂等操作号；重试需要沿用它，不能用新号绕过原操作的未知结果。',
    'Fingerprint': '规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。',
    'DisplayName': '向用户展示的昵称；可以用于列表显示，不能代替稳定账号标识做授权关联。',
    'CredentialEpoch': '凭据代次；改密码等凭据变化会推进它，旧会话与当前代次不一致时不能继续使用。',
    'IdentityEpoch': '身份状态代次；准入与恢复时比较这份身份事实是否仍对应当前账号状态。',
    'SessionExpires': '账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。',
    'SessionHash': '会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。',
    'OrganizationID': '组织的稳定标识，关联成员、角色和组织生命周期；显示名称改动不改变这个关联。',
    'UserID': '账号的稳定标识，用于查询和关系关联；昵称相同不代表同一个账号。',
    'PlanDigest': '预览/执行计划的内容摘要，确认时据此绑定原计划；不能拿新计划冒充用户此前确认的内容。',
    'ReceiptState': '原操作回执所处状态，决定是返回既有结果、继续原操作还是等待核验；未知状态不能伪报成功。',
}


def field_table(title, fields, note, detail, category):
    if not fields:
        return ''
    rows = []
    for index, field in enumerate(fields):
        name, kind = field['name'], field['type']
        explicit = detail.get(category, {})
        meaning = explicit.get(str(index)) if category == 'results' else explicit.get(name) or explicit.get(str(index))
        meaning = meaning or field.get('comment') or note.get('terms', {}).get(name) or TYPE_MEANINGS.get(kind) or TERMS.get(name)
        if not meaning and kind.endswith('Queryer'):
            meaning = 'SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。'
        if not meaning and name == '_':
            meaning = '这个位置的输入按接口约定接收，但本实现明确不使用它；下划线避免产生未使用变量。'
        if not meaning and name == '(匿名/未命名)' and category == 'fields':
            meaning = '嵌入的类型或接口，直接组合它已有的字段/方法集合；不为同一能力再写一份转发实现。'
        if not meaning:
            match = re.fullmatch(r'(?:\[\]|\*)?((?:\w+\.)?[A-Z]\w*)', kind)
            if match:
                meaning = note.get('_type_contexts', {}).get(match[1])
        evidence = []
        if field.get('tag'):
            evidence.append('序列化/校验标记：' + code(field['tag']))
        for call in field.get('forwarded') or []:
            evidence.append('由 ' + code(call) + f' 的第 {index+1} 个返回项原样转交。')
        uses = field.get('uses') or []
        if uses:
            prefix = '实际返回表达式：' if category == 'results' else '在此声明中的实际使用：'
            evidence.append(prefix + '；'.join(code(v) for v in uses[:4]))
            if len(uses) > 4:
                evidence.append(f'另有 {len(uses)-4} 处使用，继续按下方完整代码的顺序处理。')
        # 未知业务意义不根据名称臆造；具体使用证据与人工解释在不同列呈现。
        explanation = ([meaning.replace('|', '\\|').replace('\n', ' ')] if meaning else []) + evidence
        if not explanation:
            explanation = ['<!-- explanation-gap -->此字段的业务含义尚待补写。']
        rows.append('| ' + code(name) + ' | ' + code(kind) + ' | ' + '<br>'.join(explanation) + ' |')
    return f'\n{title}：\n\n| 名称 | 类型/签名 | 含义、原因与实际使用 |\n| --- | --- | --- |\n' + '\n'.join(rows) + '\n\n'


def declaration_explanation(declaration, note):
    detail = symbol_note(note, declaration)
    text = ''
    if detail.get('why'):
        text += '**这一段为什么这样写：** ' + detail['why'] + '\n\n'
    if detail.get('steps'):
        text += '\n'.join(f'{i}. {item}' for i, item in enumerate(detail['steps'], 1)) + '\n\n'
    text += field_table('字段或接口方法', declaration.get('fields'), note, detail, 'fields')
    text += field_table('输入参数', declaration.get('params'), note, detail, 'params')
    text += field_table('返回值（从左到右）', declaration.get('results'), note, detail, 'results')
    return text


def execution_facts(declaration, start, end):
    facts = list(declaration.get('facts') or [])
    for member in declaration.get('members') or []:
        facts.extend(member.get('facts') or [])
    selected = [f for f in facts if start <= f['start'] < end]
    selected.sort(key=lambda f: f['start'])
    if not selected:
        return ''
    lines = []
    last_condition = None
    for fact in selected:
        kind, value = fact['kind'], fact['text']
        if kind == 'condition':
            last_condition = fact.get('outcome')
            line = '检查 ' + code(value) + '。'
            if last_condition:
                line += '条件成立时执行 ' + code(last_condition) + '。'
                if last_condition.startswith('return'):
                    line += '当前函数在这里结束。'
                elif last_condition.startswith('continue'):
                    line += '跳过当前循环本轮剩余语句。'
                elif last_condition.startswith('break'):
                    line += '退出对应循环或switch，按代码中的标签/层级确定目标。'
                elif last_condition.startswith('throw'):
                    line += '转入外层异常处理路径。'
        elif kind == 'return':
            if last_condition and value.rstrip(';') == last_condition.rstrip(';'):
                continue
            line = '此处通过 ' + code(value) + ' 结束当前函数；注意返回的是哪一个阶段的结果。'
        elif kind == 'defer':
            line = '登记 ' + code(value) + '，当前函数退出时执行，提前 return 也会触发。'
        elif kind == 'go':
            line = '用独立 goroutine 执行 ' + code(value) + '；调用点不会等待其完成，需要下方代码中的生命周期控制。'
        elif kind == 'await':
            line = '等待 ' + code(value) + ' 完成才继续本异步函数；reject 会进入对应的错误处理路径。'
        elif kind == 'throw':
            line = '执行 ' + code(value) + '，把失败交给外层 catch 或调用者，不能继续发送成功响应。'
        else:
            line = '执行 ' + code(value) + '。'
            if '.Commit(' in value:
                line += '提交结果需要检查；收到错误不一定证明数据库没有提交。'
            elif '.BeginTx(' in value:
                line += '从这里建立事务，后续受保护读写须沿用返回的事务对象。'
            elif '.Scan(' in value:
                line += '查询列与接收变量按位置对应，调换顺序会读错数据。'
            elif '.ExecContext(' in value:
                line += '执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。'
        if fact.get('scope'):
            line = '在本段的' + fact['scope'] + '内：' + line
        if not lines or line != lines[-1]:
            lines.append(line)
    # 每段通常45行左右；保留全部条件，不用截断造成“只说明成功路径”。
    return '**跟着本段代码走一遍：**\n\n' + '\n'.join('- ' + line for line in lines) + '\n\n'
