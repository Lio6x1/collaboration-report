"""发布带写课程目录；只重排导航和保留已核对的阶段命令，不生成业务讲解。"""
import argparse
import json
from pathlib import Path
import re

HERE = Path(__file__).resolve().parent.parent
PHASES = [
    ('01','foundation','公共底座、正文契约和SQL','01-foundation.md',
     '这一阶段为后续保存正文、注册账号和跨服务操作建立基础：同一正文得到相同字节，业务数据有持久归属，服务能安全启动，断网后的操作有恢复依据。这里还没有完整用户页面。\n\n先用具体正文样例理解校验，再设计数据表、启动与错误出口、原操作协议和重建文件。F01可独立测试；F03引用F04的同包声明，写齐后运行平台规则；F05两个子包独立检查。SQL到06自己的实例才安装。',
     '正文六项和平台具名用例应实际RUN/PASS，无SKIP。各课还给出就绪、安全及重建制品反例。这里没有安装真实数据库；摘要核对不替代06的迁移、授权与恢复验收。'),
    ('02','account','账号、验证码、组织与会话','02-account.md',
     '这一阶段实现用户进入系统后的账号与团队功能：邮箱注册、登录、修改密码和邮箱、创建组织、邀请或审批成员、接收通知，以及安全退出与注销。接通后，合法用户能完成这些操作，无效凭据或不足权限会被拒绝，断网重试可以恢复原结果。\n\n先从“输入邮箱和密码怎样登录”理解一条完整链路，再扩展验证码、身份与组织管理，最后补通知、关闭和恢复。A08把业务接入HTTP、RPC和后台任务；账号包存在前向引用，整体构建到A08执行，真实邮件与页面流程在06验证。',
     '构建应退出0，四个具名规则/用例应RUN/PASS、无SKIP。弱新密码在验证码和存储前拒绝，旧密码核验保留兼容输入；数据库故障不冒充401。真实SMTP/Redis/注册与组织流程按A08和06，在自己的实例执行。'),
    ('03','document','文档、权限与完整工作流','03-document.md',
     '这一阶段实现文档从创建到协作、审核、发布和清理的完整业务。接通后，用户可管理知识库、阅读和编辑文档、拆分章节、上传图片、发表评论与导出文件；每一步都受当前权限和已确认版本约束，不能为了方便覆盖别人工作。\n\n先明确谁能操作哪份文档，再依次完成工作区、主线与支线、章节、审核、发布、图片、评论和导出，最后处理撤权、回收转移与搜索接线。D14是文档整包构建关口；转换器也依赖尚待写齐的业务声明，真实功能在06验证。',
     'domain涉及多个功能模型，须按各课把相关声明写齐后执行纯规则；D14全服务装配后再build和三个logic反例。重复块ID拒绝，合法重复图片引用保留，确定拒绝与网络未知区分。真实数据库、跨服务和转换文件在06验收。'),
    ('04','collaboration','Node协作与可靠保存','04-collaboration.md',
     '这一阶段让多人真正共同编辑：输入的内容得到可靠保存，重连后可恢复；送审和合并使用固定内容，拆章时保护在线编辑者的输入，撤权后旧连接停止工作。正文与权限的权威结果仍由文档服务配合确认。\n\n先理解一次正文更新如何检查和恢复，再连接准入、落库后的保存确认与保存证明；随后实现撤权、固定工作流、在线结构协调和清理，C07完成网络与进程装配。语法检查不等于多人保存成功，全部依赖齐备后跑规则，真实WSS与数据库在06验证。',
     '规则组应PASS、没有失败或跳过。readiness.test.mjs明确要求06的真实配置和数据库，移到真实关口执行。规则替身不能证明MySQL持久保存；真实WSS须确认落库后SAVED、去重、FLUSH和断线恢复。使用锁定Node24.19及显式no-maglev，核对进程退出码。'),
    ('05','search','正式版搜索与索引恢复','05-search.md',
     '这一阶段实现正式版文档搜索：发布后进入索引，用户按关键词查询和翻页，撤下或失去权限后不再暴露正文。服务重启能继续消费变更，索引需要重建时可追赶期间变化并安全切换。\n\n先理解发布事件如何变成可搜索内容，再保存消费进度、实现候选分页，最后准备快照、回填、追赶和切换。搜索索引是可重建副本，当前阅读资格由文档服务复核；三个存储系统不是一个事务，未知结果必须恢复原事件。S05完成全包接线，06验证真实搜索与重建。',
     '构建、domain规则和consumer具名水位测试应PASS、无SKIP。大整数身份不经float；游标核用途/精确帧/摘要；旧事件不能复活隐藏内容。此处不代表真实Canal/Kafka/ES消费完成，实际投影和搜索页面在06验证。'),
]


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--write',action='store_true')
    args=parser.parse_args()
    differences=[]
    catalog=['# 教师带写课程目录\n',
             '先完成[00空工程准备](../00-start.md)，以后始终在同一学习目录。每课先说明要实现什么功能、完成后能看到什么、准备怎样实现和何时验证；理解这些之后才进入文件分工、模型/接口、业务与SQL、接线和测试。讲到哪个文件就按对应分步页创建并录入，章末批量建文件脚本只是可选快捷方式。功能讲义与操作页中的同名声明只写一次，完整答案用于章末核对。\n',
             '“文件语法通过”“包或服务构建通过”“纯规则通过”“真实接口/存储通过”分别成立，不能互相替代。各课标明前向依赖和可执行关口，不要求写到一半时猜未来实现。\n']
    all_units=[]
    for phase,name,title,page,description,expectation in PHASES:
        units=json.loads((HERE/f'lessons/{name}-units.json').read_text(encoding='utf-8'))
        all_units.extend(units)
        rows='\n'.join(f'| {u["id"]} | [{u["title"]}]({u["page"]}) | {len(u["files"])} |' for u in units)
        original=(HERE/page).read_text(encoding='utf-8')
        gates=re.findall(r'```powershell\n[\s\S]*?\n```',original)
        if not gates: raise ValueError('PHASE_GATE_MISSING: '+page)
        text=f'# {phase} {title}：功能带写\n\n{description}\n\n'
        text+='先按下表进入功能课，跟随具体输入、字段和函数逐段落笔。[文件与完整答案索引]('+phase+'-files.md)只用于查找，表内文件序号不是教学顺序。\n\n'
        text+='| 课次 | 按功能一步一步实现 | 所属文件 |\n| --- | --- | --- |\n'+rows+'\n\n'
        text+='## 阶段检查\n\n先完成各课规定的依赖；下列为本阶段的收口命令。工作目录仍是自己的lessonRoot，GOWORK/GOCACHE按00配置；coursePrepare/lessonRoot在00设置，新开终端须重新设置。\n\n'+'\n\n'.join(gates)+'\n\n'+expectation+'\n\n'
        text+='缺声明先查本阶段分步是否漏写；重复声明检查是否把完整答案又追加一遍。不可用、超时或未知结果按各课说明保留原操作，不能以删断言、跳过测试或另造新号制造成功。真实功能和页面按[06完整装配](06-integration.md)继续。\n'
        if text!=original:
            differences.append(page)
            if args.write: (HERE/page).write_text(text,encoding='utf-8',newline='\n')
        catalog.append(f'## {phase} {title}\n\n{description}\n\n| 课次 | 功能讲义 | 亲写文件 |\n| --- | --- | --- |\n'+rows.replace('(lessons/','(')+'\n')
    catalog.append('## 完整联调\n\n完成全部39课后进入[06自己的实例与真实功能关口](../06-integration.md)，复用现成前端逐模块验证。再用[07练习](../07-exercises.md)检查能否解释和修改；[旧工程证据](../08-verification.md)与[新带写验证](../09-teaching-verification.md)分开阅读。\n')
    catalog.append('讲义所属文件机器清单是各个 `*-units.json`；逐段代码边界在 `write-map.json`。清单只能证明覆盖和材料一致，教学解释还须按用户五点逐课核对。\n')
    path=HERE/'lessons/README.md'
    text='\n'.join(catalog)
    if not path.exists() or path.read_text(encoding='utf-8')!=text:
        differences.append('lessons/README.md')
        if args.write: path.write_text(text,encoding='utf-8',newline='\n')
    print(json.dumps({'state':'WRITTEN' if args.write else 'PASS' if not differences else 'STALE',
                      'units':len(all_units),'files':sum(len(u['files']) for u in all_units),
                      'differences':differences},ensure_ascii=False))
    if differences and not args.write: raise SystemExit(1)


if __name__=='__main__': main()
