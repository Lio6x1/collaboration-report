# Windows Node 退出诊断与可选兼容配置

本记录区分原生退出、监督器就绪失败与业务失败。兼容配置不代表根因已经证明，也不代替完整第四批验收。

## 已观察事实

第五轮完整第四批在提交 `297d030`、dirty=false 上执行，公共检查、实际底座 32 项和协作 11 项通过。审核组共有 138 个报告项（129 个顶层用例及 9 个嵌套用例），45 PASS、93 FAIL、0 SKIP。前 36 个顶层用例通过，下一项连接重置，其后连接拒绝。只读 PID 核验发现 Edge 已不存在，其余五个角色仍在；没有取得该次 Edge 的退出码，不把它归类为已证实的原生异常。浏览器组未运行。

原回执和失败日志保留在 `artifacts/batches/04/verification-attempt-05.json`、`full-verification-05.txt` 和 `review-main-api-fifth-failed.tap`。owner 恢复记录为 `owner-recovery-after-attempt05-stop.txt`、`owner-recovery-after-attempt05-start.txt`。同一恢复组合的隔离复验 `discard-recovery-edge-exit-reproduction-01.tap` 为 3/3 PASS；它不能抹掉整轮失败。

后续余项运行 `review-remaining-with-exit-watch-01.tap` 为 6/93 PASS，首个失败是 `discard-preview-context-change` 的 finally 重启。监督器退出 1，末阶段为 PROBE_FAILED，稳定码 SERVICE_READINESS_TIMEOUT；监测发现它终止了新启动的三个 Node 角色。另一次同一业务样本 `discard-context-startup-profile-01.tap` 的监督器实际退出码为 `3221226505`（`0xC0000409`），没有 JS 栈或原生断言文本；该次子服务仍存活，协作服务之后完成了真实房间恢复并开始监听。二者是不同失败，不能混为一种原因。

可选阶段测量显示默认 V8 下既存房间校验约 14～15 秒，关闭 Maglev 的对照约 16.8 秒。这份业务计时从 `run()` 开始，不包含模块导入；它证明房间恢复占用主要启动时间，尚未证明所有就绪失败由此造成。全部房间及可靠日志仍逐项核验，原预算不变。

## 对照与正式参数

早期只读探针对照默认/关闭 Maglev 各 10 轮全通过，没有区分能力。后续改为相同实际恢复工作负载：自然证明到期与 owner 重启、已成功安装责任及注销恢复、丢弃审核、原控制版本变化。诊断用 `no-maglev-recovery-actual-comparison-01.tap` 为 4/4 PASS、0 SKIP，耗时约 185 秒；只改变 Node CLI 优化器参数。首个 shim 启动因 Windows import 路径格式错误失败，记录保留，不算业务证据。

正式方案不依赖诊断 shim。`runtime-lib.nodeArguments` 只继承调用者明确给出的 `--no-maglev`，覆盖监督启动、Node 子服务和恢复助手；不转发测试/导入/执行参数。真实两层 Node 启动反例先观察到选项丢失，见 `node-runtime-flags-red.tap`，修复后的根测试 `node-runtime-unit-green-02.tap` 为 38/38 PASS、0 SKIP。初次受沙箱 WMI 拒绝的两份失败日志也保留；实际 Windows 身份测试不能把权限拒绝当作 PID 已退出。

正式启动 `formal-no-maglev-start-01.txt` 退出 0 并完成准确 owner/READY 核验。CIM 布尔核验确认 SMTP、协作服务、Edge 均实际带 `--no-maglev`，均无诊断 shim，owner 匹配。正式实际恢复复验及完整第四批的最终结果分别追加到交付台账，不预写 PASS。

Windows 命令见 [WINDOWS.md](WINDOWS.md)。整批回执保存实际 Node 版本和显式 `nodeFlags`。默认行为不改变，不将不受支持的 CLI 参数塞入 `NODE_OPTIONS`。

[Node 官方问题 62260](https://github.com/nodejs/node/issues/62260) 报告了另一版本 Node 在相似 Windows 环境中的 Maglev/原生退出现象，仅作为候选依据；本项目没有取得足以证明同一根因的崩溃栈。

## 第六轮与完整房间恢复的等待问题

第六轮在 `88bf1ac`、dirty=false、Node 24.15.0、显式 `--no-maglev` 上运行。公共检查、实际 integration 32 项及协作 11 项通过；review 50/138 PASS、88 FAIL、0 SKIP。第 7 个顶层固定捕获重启用例与第 43 个顶层控制上下文变化用例的监督启动均退出 1，稳定码 SERVICE_READINESS_TIMEOUT；后者使其余用例连接拒绝，浏览器未运行。原回执保存为 `verification-attempt-06.json`，原审核日志为 `review-main-api-sixth-failed.tap`。这说明关闭 Maglev 不能单独解决当前全部启动失败。

失败前的业务阶段记录显示全部既存房间校验约 21.8 秒才完成。实际源码 `RoomSnapshots.verifyRooms` 为每个房间依次等待检查点和日志查询，全部房间的独立读 I/O 被串行累计。原就绪轮询次数和暂停间隔不变；启动失败后 owner 核验和清理仍可能让协作服务在终止前继续完成恢复，不能把稍后出现监听日志当作本次启动成功。

两个固定恢复通道的方案先通过隔离反例：首个房间的查询被阻塞时第二个尚未开始；损坏检查点必须拒绝、不能被日志掩盖；退役房间的实际日志缺口仍致命。SQL 适配器为单元隔离，但检查点、checksum、Yjs 回放和内容规则沿真实恢复函数。正式 `room-recovery-bounded-red.tap` 为 1 PASS、2 FAIL，修改后 3/3 PASS；根与原 Node 服务联合回归 `room-recovery-unit-service-green.tap` 为 115/115 PASS、0 SKIP。

恢复仍覆盖全部房间，包括退役房间；每房间依次检查 checkpoint→连续日志→内容，不重排其内部验证。两个通道限制同时载入的正文数量，任一路失败后停止领取新房间，并等待已开始的恢复释放资源再抛出原失败，全部通过才设置 roomsRecovered。没有扩大任何 SQL/RPC/HTTP/就绪预算、缩减数据集或清理历史事实。实际 owner 恢复 `owner-recovery-after-attempt06-bounded.txt` 退出 0 并 READY；这次完整房间阶段在 9.4 秒完成。该单次测量不作为 P95 或全部故障根因证明，后续实际恢复与完整验收仍须通过。

诊断监测 03 使用 PowerShell 5 时误将 JSON 数组包成双层数组，只记录到 SMTP，不能引用其不存在的另外两个角色退出证据。旧监测 01/02 的三个角色记录已核验。监测脚本恢复单层数组读取后，监测 04 已实际持有 SMTP、协作和 Edge 的指定 PID 句柄，后续只读监测与业务验收独立。

`room-recovery-bounded-actual-01.tap` 随后实际 6/6 PASS、0 SKIP（232 秒），包含第六轮原两个失败的真实重启及可靠日志、固定捕获、自然失效与安装责任恢复。全部原完整正文/序号/原号/SHA/epoch断言保留。这是有界恢复的受影响证据；完整第七轮仍需独立执行，不据此预写整批 PASS。

## 第七轮原生退出及24.19.0候选运行时

第七轮实际Edge PID句柄于2026-10-06 15:44:36 UTC取得退出码 `-1073740791`/`0xC0000409`，本进程此前确带`--no-maglev`，其他五个角色仍存活。日志没有原生断言或崩溃栈，Windows事件查询没有匹配的可用记录，不能声称关闭Maglev已经消除此类原生退出，也不能认定所有同码退出具有同一原因。

项目内ignored `.runtime/toolchains/node-v24.19.0-win-x64`来自[Node官方24.19.0分发](https://nodejs.org/dist/v24.19.0/)，ZIP的同源SHASUMS256核验值为`57f71ab3652e797d84acddc79c81cc9ff1c6ddb2a1974cdb83f00fee9bff4c73`；exe核验值为`3602f2bb1a10f2cbab4c36886218a33c1ab3db87290e73b033c46c77147d0237`，与本机Codex捆绑24.19.0的exe相同。正式试验使用完整官方分发及当前会话PATH，没有全局安装或系统变更。分发事实另存`node-24-19-distribution.json`。

相同源码原PULL恢复两项在24.19.0实际通过，不足以证明原生根因已解决。第八轮使用这一运行时，审核133/138 PASS、5 FAIL、0 SKIP；四项readiness 503和一个关闭时序问题待诊断，浏览器未执行。本轮句柄监测没有新增原生退出记录，但整轮仍FAIL，24.19.0尚未作为完整验收通过的基线定稿。
