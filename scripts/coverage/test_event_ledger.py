import unittest,random,json,hashlib,pathlib,tempfile,copy
import event_ledger as r
class Contracts(unittest.TestCase):
 def setUp(self):
  self.raw=b'package a\n\tvar '+ '名稱'.encode()+b' = 1\n';self.source={'a.go':self.raw};self.site={'module':'example/a','version':'workspace','context':'owned-example','source_sha256':hashlib.sha256(self.raw).hexdigest(),'file':'a.go','line':2,'column':6,'phase':'entry','adapter':'test','context':'owned-example'};self.contexts={('example/a','workspace','owned-example')}
 def test_utf8_byte_column_golden(self):
  for column in [1,2,3,4,5,6,9,12,13,14,15,16]:
   s=dict(self.site,column=column);r.validate_site(s,self.source,self.contexts)
  for column in [7,8,10,11,17,0,True]:
   with self.assertRaises(ValueError):r.validate_site(dict(self.site,column=column),self.source,self.contexts)
 def test_multibyte_generated_property(self):
  rng=random.Random(999)
  for _ in range(500):
   text='\t'+''.join(rng.choice(['a','名','稱','é','🙂','\t']) for _ in range(rng.randrange(1,25)))
   raw=(text+'\n').encode();sources={'unicode.go':raw};site=dict(self.site,file='unicode.go',source_sha256=hashlib.sha256(raw).hexdigest(),line=1)
   boundaries={len(text[:n].encode())+1 for n in range(len(text)+1)}
   for column in range(1,len(raw)+1):
    if column in boundaries:r.validate_site(dict(site,column=column),sources,self.contexts)
    else:
     with self.assertRaises(ValueError):r.validate_site(dict(site,column=column),sources,self.contexts)
 def test_paths_and_context(self):
  for path in ['../a.go','/a.go','a/../a.go','a\\b.go','a\x00.go','./a.go','a//b.go']:
   with self.assertRaises(ValueError):r.relative_file(path)
   with self.assertRaises(ValueError):r.validate_site(dict(self.site,file=path),self.source,self.contexts)
  with self.assertRaises(ValueError):r.validate_site(dict(self.site,version='released'),self.source,self.contexts)
 def test_workspace_symlink_escape(self):
  with tempfile.TemporaryDirectory() as tmp,tempfile.TemporaryDirectory() as foreign:
   pathlib.Path(foreign,'a.go').write_bytes(self.raw);pathlib.Path(tmp,'a.go').symlink_to(pathlib.Path(foreign,'a.go'))
   with self.assertRaises(ValueError):r.source_bytes(tmp,'a.go')
 def test_identity_and_other_context_smuggling(self):
  a=r.validate_site(self.site,self.source,self.contexts);other=dict(self.site,adapter='ServeMux');b=r.validate_site(other,self.source,self.contexts);registry={a:self.site,b:other};record={'id':a,'pid':123,'kind':'base-fatal'};processes={(123,'base-fatal'):{a}}
  self.assertEqual(r.ledger(registry,[record],processes)['event_counts'],{a:1})
  with self.assertRaisesRegex(ValueError,'outside compiled process'):r.ledger(registry,[dict(record,id=b)],processes)
  for changed in [dict(record,pid=True),dict(record,id='unknown'),dict(record,kind='unknown'),dict(record,extra=1)]:
   with self.assertRaises(ValueError):r.ledger(registry,[changed],processes)
 def test_pbt_generated_immutable_identity(self):
  rng=random.Random(20261001);key=r.validate_site(self.site,self.source,self.contexts)
  for _ in range(500):
   items=list(self.site.items());rng.shuffle(items);self.assertEqual(r.validate_site(dict(items),self.source,self.contexts),key)
   changed=dict(self.site,line=rng.choice([-1,0,3,99,True]),column=rng.randrange(-50,51))
   with self.assertRaises(ValueError):r.validate_site(changed,self.source,self.contexts)
 def test_bounded_malformed_json_fuzz(self):
  rng=random.Random(124);invalid=['{"x":1,"x":2}','{"x":NaN}','[','{','null trailing']
  for _ in range(1000):invalid.append('{' + ''.join(rng.choice('xyz\\\n:\"') for _ in range(rng.randrange(30))))
  for raw in invalid:
   with self.assertRaises((ValueError,json.JSONDecodeError)):r.strict_json(raw)

class CLIContracts(unittest.TestCase):
 def test_real_cli_boundaries_and_generated_shapes(self):
  import subprocess,sys
  with tempfile.TemporaryDirectory() as td:
   root=pathlib.Path(td);raw=b'package a\nfunc f() {}\n';(root/'a.go').write_bytes(raw);mod=b'module example/a\n';(root/'go.mod').write_bytes(mod);(root/'producer.txt').write_text('immutable producer')
   site={'module':'example/a','version':'workspace','context':'owned','source_sha256':hashlib.sha256(raw).hexdigest(),'file':'a.go','line':2,'column':1,'phase':'entry','adapter':'a','context':'owned'};key=r.digest(site);other=dict(site,adapter='foreign');foreign=r.digest(other);registry={key:site,foreign:other};record={'id':key,'pid':42,'kind':'base-fatal'};journal=(json.dumps(record)+'\n').encode();(root/'journal.jsonl').write_bytes(journal)
   sha=lambda p:hashlib.sha256((root/p).read_bytes()).hexdigest()
   manifest={'registry_sha256':'','compiled_contexts':[{'module':'example/a','version':'workspace','context':'owned','go_mod_file':'go.mod','go_mod_sha256':sha('go.mod')}],'producer_materials':[{'file':'producer.txt','sha256':sha('producer.txt')}],'processes':[{'pid':42,'kind':'base-fatal','terminal':True,'exit':7,'expected_exit':7,'disposition':'expected-fatal','allowed_site_ids':[key],'producer_sha256':sha('producer.txt')}],'journals':[{'file':'journal.jsonl','sha256':sha('journal.jsonl'),'pid':42,'kind':'base-fatal'}],'unsupported':{'physical_inventory':'not adopted denominator'}}
   (root/'alias.jsonl').symlink_to(root/'journal.jsonl')
   cases=['positive','registry-list','manifest-list','context-list','journal-list','journal-null','duplicate-process','duplicate-journal','alias-journal','foreign-id','bool-process','bool-journal']
   rng=random.Random(761);cases += [('generated',rng.choice(['compiled_contexts','processes','journals','producer_materials']),rng.choice([None,{},1,True,'x'])) for _ in range(80)]
   for case in cases:
    reg=copy.deepcopy(registry);m=copy.deepcopy(manifest);j=journal
    if case=='registry-list':reg=[]
    elif case=='manifest-list':m=[]
    elif case=='context-list':m['compiled_contexts']=[[]]
    elif case in ['journal-list','journal-null']:j=b'[]\n' if case=='journal-list' else b'null\n'
    elif case=='duplicate-process':m['processes'].append(copy.deepcopy(m['processes'][0]))
    elif case=='duplicate-journal':m['journals'].append(copy.deepcopy(m['journals'][0]))
    elif case=='alias-journal':m['journals'].append(dict(m['journals'][0],file='alias.jsonl'))
    elif case=='foreign-id':j=(json.dumps(dict(record,id=foreign))+'\n').encode()
    elif case=='bool-process':m['processes'][0]['pid']=True
    elif case=='bool-journal':m['journals'][0]['pid']=True
    elif isinstance(case,tuple):m[case[1]]=case[2]
    (root/'journal.jsonl').write_bytes(j);(root/'registry.json').write_text(json.dumps(reg))
    if isinstance(m,dict):
     m['registry_sha256']=sha('registry.json')
     if isinstance(m['journals'],list):
      for row in m['journals']:
       if isinstance(row,dict):row['sha256']=sha('journal.jsonl')
    (root/'manifest.json').write_text(json.dumps(m));output=root/'output.json';output.unlink(missing_ok=True)
    cmd=[sys.executable,str(pathlib.Path(r.__file__)), '--source-root',str(root),'--registry',str(root/'registry.json'),'--manifest',str(root/'manifest.json'),'--output',str(output)]
    p=subprocess.run(cmd,capture_output=True,timeout=5)
    with self.subTest(case=case):
     if case=='positive':
      self.assertEqual(p.returncode,0);self.assertEqual(p.stderr,b'');self.assertEqual(json.loads(output.read_text())['event_counts'],{key:1})
     else:
      self.assertEqual(p.returncode,2);self.assertEqual(p.stderr,b'observer-consumer: invalid bound evidence\n');self.assertFalse(output.exists())

if __name__=='__main__':unittest.main()
