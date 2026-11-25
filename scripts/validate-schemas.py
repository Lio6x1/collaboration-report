"""验证正式内容schema及迁移过来的合法字节样本；不启动历史实验。"""
import json, hashlib
from pathlib import Path
from jsonschema import Draft202012Validator
root=Path(__file__).resolve().parent.parent
folder=root/'contracts'/'content'
for name in ('content.schema.json','provenance.schema.json'):
    Draft202012Validator.check_schema(json.loads((folder/name).read_text(encoding='utf-8')))
validator=Draft202012Validator(json.loads((folder/'content.schema.json').read_text(encoding='utf-8')))
vectors=json.loads((folder/'canonical-vectors.json').read_text(encoding='utf-8'))['vectors']
for v in vectors:
    validator.validate(v['input'])
    raw=v['canonical_utf8'].encode('utf-8')
    assert len(raw)==v['byte_size'] and hashlib.sha256(raw).hexdigest()==v['sha256'], v['name']
print(json.dumps({'schemas':2,'vectors':len(vectors),'state':'PASS'}))
