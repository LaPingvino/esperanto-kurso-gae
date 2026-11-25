import json,sys,re,html
from html.parser import HTMLParser
class P(HTMLParser):
    def __init__(s): super().__init__(); s.out=[]; s.skip=0
    def handle_starttag(s,t,a):
        a=dict(a)
        if t in('style','script') or 'pagenum' in (a.get('class') or '') or 'ws-pagenum' in (a.get('class') or '') or a.get('class','').startswith('mw-editsection') or 'ws-noexport' in (a.get('class') or ''): s.skip+=1; s.stack=getattr(s,'stack',[])+[t]; return
        if s.skip: s.stack.append(t); return
        if t in('p','br','div','tr','h1','h2','h3','li','dd'): s.out.append('\n')
    def handle_endtag(s,t):
        if s.skip:
            s.stack.pop()
            if not s.stack: s.skip=0
            return
        if t in('p','div'): s.out.append('\n')
    def handle_data(s,d):
        if not s.skip: s.out.append(d)
for f in sys.argv[1:]:
    h=json.load(open(f))['parse']['text']['*']
    p=P(); p.feed(h)
    t=re.sub(r'\n\s*\n+','\n\n',''.join(p.out))
    open(f.replace('.json','.txt'),'w').write(t)
    print(f, len(t))
