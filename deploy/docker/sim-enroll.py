#!/usr/bin/env python3
import base64,json,os,subprocess,sys,tempfile,time,urllib.request,urllib.error,http.cookiejar
MASTER="http://127.0.0.1:9999"; USER="admin"; PASS="e55386136f641e811fafa781"; PUB="/tmp/panel_pub.pem"
cj=http.cookiejar.CookieJar(); op=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))
def sh(a,d=None):
    p=subprocess.run(a,input=d,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if p.returncode: raise RuntimeError(p.stderr.decode())
    return p.stdout
def envelope(pw):
    k=os.urandom(32); iv=os.urandom(16)
    with tempfile.TemporaryDirectory() as t:
        kf=t+"/k"; open(kf,"wb").write(k); pf=t+"/p"; open(pf,"wb").write(pw.encode())
        ct=sh(["openssl","enc","-aes-256-cbc","-nosalt","-K",k.hex(),"-iv",iv.hex(),"-in",pf])
        rsa=sh(["openssl","rsautl","-encrypt","-pubin","-inkey",PUB,"-pkcs","-in",kf])
    e=base64.b64encode
    return f"{e(rsa).decode()}:{e(iv).decode()}:{e(ct).decode()}"
def cook(n):
    for c in cj:
        if c.name==n: return c.value
def req(path,body=None,method=None):
    r=urllib.request.Request(MASTER+path,
        data=json.dumps(body).encode() if body is not None else None,
        method=method or ("POST" if body is not None else "GET"))
    r.add_header("Content-Type","application/json")
    t=cook("pcsrftoken")
    if t: r.add_header("X-CSRF-Token",t)
    try:
        x=op.open(r,timeout=320); return x.status,x.read().decode()
    except urllib.error.HTTPError as e: return e.code,e.read().decode()

print("login:",req("/api/v2/core/auth/login",{"name":USER,"password":envelope(PASS),"language":"en"})[0])
st,bd=req("/api/v2/core/nodes/search",{"page":1,"pageSize":50}); d=json.loads(bd).get("data",{})
for it in d.get("items",[]) or []:
    print("delete stale node",it["id"],it["name"],"->",req(f"/api/v2/core/nodes/del/{it['id']}",{} )[0])
for n in ("node-a","node-b"):
    st,bd=req("/api/v2/core/nodes/add",{"name":n,"addr":n,"port":9999,"sshUser":"root","sshPort":22,"sshPassword":"nodepass123","description":"sim"})
    print(f"add {n}: {st}", json.loads(bd).get("data",{}).get("status") if st==200 else bd[:200])
print("waiting 30s for slave agents to init + bind mTLS:9999 ...")
time.sleep(30)
st,bd=req("/api/v2/core/nodes/search",{"page":1,"pageSize":50}); items=json.loads(bd)["data"]["items"]
for it in items:
    rc,rb=req(f"/api/v2/core/nodes/healthcheck/{it['id']}",{})
    j=json.loads(rb).get("data",{}) if rc==200 else {}
    print(f"healthcheck {it['name']}: http{rc} status={j.get('status')} ver={j.get('version')} msg={(j.get('lastMessage') or '')[:90]}")
# mTLS reverse-proxy proof: hit an agent endpoint THROUGH the master, selecting node-a
print("== mTLS proxy test (CurrentNode: node-a) ==")
r=urllib.request.Request(MASTER+"/api/v2/health/check"); r.add_header("CurrentNode","node-a")
t=cook("pcsrftoken")
if t: r.add_header("X-CSRF-Token",t)
try:
    x=op.open(r,timeout=30); print("proxied /api/v2/health/check via node-a ->",x.status,x.read().decode()[:160])
except urllib.error.HTTPError as e: print("proxied ->",e.code,e.read().decode()[:200])
