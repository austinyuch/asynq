"""Source-bound experimental registry/journal consumer; no line-ratio policy."""
import argparse,hashlib,json,pathlib,re,sys
PHASES={'entry','completion','effect'}
KINDS={'base-parent','base-fatal','mux-parent','timeutil-parent'}
def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def relative_file(value):
 if not isinstance(value,str) or not value or '\\' in value or '\x00' in value:raise ValueError('file path')
 p=pathlib.PurePosixPath(value)
 if p.is_absolute() or any(v in {'..','.'} for v in value.split('/')) or str(p)!=value:raise ValueError('file path')
 return p
def source_bytes(root,name):
 relative_file(name);root=pathlib.Path(root).resolve(strict=True);p=root.joinpath(name).resolve(strict=True)
 if not p.is_relative_to(root) or not p.is_file():raise ValueError('source escapes root')
 return p.read_bytes()
def validate_site(site,sources,contexts=None):
 required={'module','version','source_sha256','file','line','column','phase','adapter','context'}
 if not isinstance(site,dict) or set(site)!=required:raise ValueError('site keys')
 if not all(isinstance(site[k],str) and site[k] for k in required-{'line','column'}):raise ValueError('site string fields')
 relative_file(site['file'])
 if site['phase'] not in PHASES:raise ValueError('phase')
 if not re.fullmatch('[0-9a-f]{64}',site['source_sha256']):raise ValueError('source sha')
 if contexts is not None and (site['module'],site['version'],site['context']) not in contexts:raise ValueError('compiled context')
 if site['file'] not in sources:raise ValueError('unknown source')
 raw=sources[site['file']]
 if hashlib.sha256(raw).hexdigest()!=site['source_sha256']:raise ValueError('changed source')
 rows=raw.splitlines()
 if type(site['line']) is not int or type(site['column']) is not int or not 1<=site['line']<=len(rows) or not 1<=site['column']<=len(rows[site['line']-1])+1:raise ValueError('byte position')
 # Go token positions cannot split a UTF-8 codepoint. Tabs count as one byte.
 try:rows[site['line']-1][:site['column']-1].decode('utf-8')
 except UnicodeDecodeError:raise ValueError('byte column splits UTF8')
 return digest(site)
def ledger(registry,records,processes=None):
 result={}
 for record in records:
  if not isinstance(record,dict) or set(record)!= {'id','pid','kind'} or type(record['pid']) is not int or record['pid']<=0 or not isinstance(record['kind'],str) or record['kind'] not in KINDS:raise ValueError('journal shape')
  if processes is not None:
   if (record['pid'],record['kind']) not in processes:raise ValueError('process identity')
   if record['id'] not in processes[(record['pid'],record['kind'])]:raise ValueError('site outside compiled process')
  if record['id'] not in registry:raise ValueError('unknown site ID')
  result[record['id']]=result.get(record['id'],0)+1
 phases={phase:sorted({(registry[i]['module'],registry[i]['version'],registry[i]['source_sha256'],registry[i]['file'],registry[i]['line']) for i in result if registry[i]['phase']==phase}) for phase in PHASES}
 return {'event_counts':result,'actual_source_lines_by_phase':phases,'line_metric_policy':'separate entry/completion/effect; no adopted global executable-line denominator or ratio'}
def strict_json(raw):
 def pairs(values):
  d={}
  for k,v in values:
   if k in d:raise ValueError('duplicate JSON key')
   d[k]=v
  return d
 return json.loads(raw,object_pairs_hook=pairs,parse_constant=lambda value:(_ for _ in ()).throw(ValueError('nonfinite JSON')))
def consume(root,registry_path,manifest_path):
 registry=strict_json(pathlib.Path(registry_path).read_bytes());manifest=strict_json(pathlib.Path(manifest_path).read_bytes())
 if not isinstance(registry,dict) or not isinstance(manifest,dict):raise ValueError('top-level shape')
 if set(manifest)!={'registry_sha256','compiled_contexts','processes','journals','unsupported','producer_materials'}:raise ValueError('manifest shape')
 if hashlib.sha256(pathlib.Path(registry_path).read_bytes()).hexdigest()!=manifest['registry_sha256']:raise ValueError('registry binding')
 for field in ['compiled_contexts','processes','journals','producer_materials']:
  if not isinstance(manifest[field],list):raise ValueError('manifest collection shape')
 contexts=set()
 for c in manifest['compiled_contexts']:
  if not isinstance(c,dict):raise ValueError('context row shape')
  if set(c)!={'module','version','context','go_mod_file','go_mod_sha256'}:raise ValueError('context shape')
  if not all(isinstance(v,str) and v for v in c.values()):raise ValueError('context fields')
  mod=source_bytes(root,c['go_mod_file'])
  if hashlib.sha256(mod).hexdigest()!=c['go_mod_sha256'] or not re.search(rb'^module '+re.escape(c['module'].encode())+rb'\s*$',mod,re.M):raise ValueError('go.mod context binding')
  contexts.add((c['module'],c['version'],c['context']))
 if not all(isinstance(k,str) and isinstance(s,dict) and isinstance(s.get('file'),str) for k,s in registry.items()):raise ValueError('registry row shape')
 sources={s['file']:source_bytes(root,s['file']) for s in registry.values()}
 for key,site in registry.items():
  if validate_site(site,sources,contexts)!=key:raise ValueError('site ID mismatch')
 producer_hashes=set()
 base=pathlib.Path(manifest_path).resolve().parent
 for material in manifest['producer_materials']:
  if not isinstance(material,dict):raise ValueError('producer row shape')
  if set(material)!= {'file','sha256'}:raise ValueError('producer material shape')
  relative_file(material['file']);path=(base/material['file']).resolve(strict=True)
  if not path.is_relative_to(base) or hashlib.sha256(path.read_bytes()).hexdigest()!=material['sha256']:raise ValueError('producer material binding')
  producer_hashes.add(material['sha256'])
 processes={}
 for p in manifest['processes']:
  if not isinstance(p,dict):raise ValueError('process row shape')
  if set(p)!={'pid','kind','terminal','exit','expected_exit','disposition','allowed_site_ids','producer_sha256'} or type(p['pid']) is not int or p['pid']<=0 or not isinstance(p['kind'],str) or p['kind'] not in KINDS or p['terminal'] is not True or type(p['exit']) is not int:raise ValueError('execution identity')
  if not isinstance(p['kind'],str) or not isinstance(p['producer_sha256'],str):raise ValueError('process fields')
  if p['producer_sha256'] not in producer_hashes:raise ValueError('unbound producer')
  if type(p['expected_exit']) is not int or p['exit']!=p['expected_exit'] or p['disposition'] not in ['normal-pass','expected-fatal'] or not isinstance(p['allowed_site_ids'],list) or not p['allowed_site_ids'] or not re.fullmatch('[0-9a-f]{64}',p['producer_sha256']):raise ValueError('execution disposition')
  if p['kind']=='base-fatal' and (p['expected_exit']!=7 or p['disposition']!='expected-fatal'):raise ValueError('fatal disposition')
  if p['kind']!='base-fatal' and (p['expected_exit']!=0 or p['disposition']!='normal-pass'):raise ValueError('normal disposition')
  if not all(isinstance(i,str) for i in p['allowed_site_ids']):raise ValueError('process site IDs')
  if (p['pid'],p['kind']) in processes:raise ValueError('duplicate process identity')
  if not set(p['allowed_site_ids']).issubset(registry):raise ValueError('process site scope')
  processes[(p['pid'],p['kind'])]=set(p['allowed_site_ids'])
 records=[];seen_journals=set();base=pathlib.Path(manifest_path).resolve().parent
 for item in manifest['journals']:
  if not isinstance(item,dict):raise ValueError('journal row shape')
  if set(item)!= {'file','sha256','pid','kind'}:raise ValueError('journal material shape')
  if type(item['pid']) is not int or item['pid']<=0 or not isinstance(item['kind'],str) or item['kind'] not in KINDS:raise ValueError('journal identity fields')
  relative_file(item['file']);p=(base/item['file']).resolve(strict=True)
  if not p.is_relative_to(base) or hashlib.sha256(p.read_bytes()).hexdigest()!=item['sha256']:raise ValueError('journal material binding')
  if p in seen_journals:raise ValueError('duplicate canonical journal')
  seen_journals.add(p)
  if (item['pid'],item['kind']) not in processes:raise ValueError('journal process binding')
  for line in p.read_bytes().splitlines():
   record=strict_json(line)
   if not isinstance(record,dict):raise ValueError('journal record shape')
   if record.get('pid')!=item['pid'] or record.get('kind')!=item['kind']:raise ValueError('journal mismatched producer')
   records.append(record)
 result=ledger(registry,records,processes);result['typed_unsupported']=manifest['unsupported'];result['trust_boundary']='trusted immutable source/evidence roots and trusted execution-manifest producer; hashes/compiled-context assertions verified for internal consistency, not external authentication or adversarial mutable-root/TOCTOU protection';return result
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--source-root',required=True);parser.add_argument('--registry',required=True);parser.add_argument('--manifest',required=True);parser.add_argument('--output',required=True);args=parser.parse_args()
 try:r=consume(args.source_root,args.registry,args.manifest);pathlib.Path(args.output).write_text(json.dumps(r,indent=2)+'\n')
 except (ValueError,OSError,KeyError,TypeError,json.JSONDecodeError):print('observer-consumer: invalid bound evidence',file=sys.stderr);sys.exit(2)
