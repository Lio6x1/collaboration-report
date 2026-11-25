"""只校验正式运行时响应，不重启或执行技术验证阶段的实验。"""
import json, sys
from pathlib import Path
from jsonschema import Draft202012Validator
root=Path(__file__).resolve().parent.parent
# 与TypeScript生成器和运行时测试共用唯一扩展合并器，避免Python重复白名单漏掉新字段。
# 只读取公开契约；不访问配置、网络或实例数据。
import subprocess
api=json.loads(subprocess.check_output([
    'node', '--no-maglev', '--input-type=module', '-e',
    "import {loadHTTPContract} from './scripts/load-http-contract.mjs'; process.stdout.write(JSON.stringify(loadHTTPContract()));"
], cwd=root))
def openapi_schema(value):
    # OpenAPI3.0的nullable不是JSONSchema2020关键字，按原契约转为显式null类型。
    if isinstance(value,list):
        return [openapi_schema(v) for v in value]
    if not isinstance(value,dict):
        return value
    result={k:openapi_schema(v) for k,v in value.items() if k!='nullable'}
    if value.get('nullable'):
        if 'type' in result:
            result['type']=[result['type'],'null']
        else:
            result={'anyOf':[result,{'type':'null'}]}
    return result
api=openapi_schema(api)
# Node的匿名管道固定发送UTF-8。Windows sys.stdin的本地代码页可能是GBK，
# 必须按协议解码原字节，不能改写中文/emoji后才验字符数与正文schema。
checks=json.loads(sys.stdin.buffer.read().decode('utf-8'))
for check in checks:
    name=check['schema']
    if name in ('content','provenance'):
        schema=json.loads((root/'contracts/content'/f'{name}.schema.json').read_text(encoding='utf-8'))
    else:
        schema={'$ref':f'#/components/schemas/{name}','components':api['components']}
    errors=list(Draft202012Validator(schema).iter_errors(check['value']))
    if errors:
        print(json.dumps({'code':'RESPONSE_SCHEMA_INVALID','model':name,'path':list(errors[0].absolute_path)}))
        sys.exit(1)
print(json.dumps({'state':'PASS','responses':len(checks)}))
