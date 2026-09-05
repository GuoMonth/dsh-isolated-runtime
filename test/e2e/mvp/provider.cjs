// Only the external model HTTP boundary is deterministic; DSH owns every tool.
const http = require('node:http');
const crypto = require('node:crypto');
const evidence = {requests:0,writeResults:0,readResults:0,attachmentReads:0,images:0,models:[]};
const files = new Map();
http.createServer(async (req,res) => {
  const json=value=>{res.setHeader('content-type','application/json');res.end(JSON.stringify(value));};
  try {
    const url=new URL(req.url,'http://fixture');
    if(url.pathname==='/evidence')return json(evidence);
    if(url.pathname.endsWith('/models'))return json({object:'list',data:[{id:'deepseek-v4-flash-vision-exp',object:'model'}]});
    let raw=Buffer.alloc(0);for await(const chunk of req) {raw=Buffer.concat([raw,chunk]);if(raw.length>8*1024*1024)throw Error('request too large');}
    if(url.pathname.endsWith('/files') && req.method==='POST') {
      const form=await new Response(raw,{headers:{'content-type':req.headers['content-type']}}).formData();
      const upload=form.get('file');const bytes=Buffer.from(await upload.arrayBuffer());
      if(bytes.length===0)throw Error('empty image upload');
      const id='file-'+crypto.createHash('sha256').update(bytes).digest('hex');
      const file={id,object:'file',bytes:bytes.length,filename:upload.name,purpose:'user_data',created_at:Math.floor(Date.now()/1000),expires_at:Math.floor(Date.now()/1000)+86400};
      files.set(id,file);return json(file);
    }
    if(url.pathname.includes('/files/')) {
      const file=files.get(url.pathname.split('/').at(-1));
      if(!file)throw Error('unknown provider image');return json(file);
    }
    if(!url.pathname.endsWith('/chat/completions')) {res.writeHead(404);res.end();return;}
    const body=JSON.parse(raw);evidence.requests++;
    if(!evidence.models.includes(body.model))evidence.models.push(body.model);
    const messages=body.messages||[];
    for(const message of messages)for(const part of Array.isArray(message.content)?message.content:[]) {
      if(part.type==='file') {if(!files.has(part.file_id))throw Error('image bytes never uploaded');evidence.images++;}
      if(part.type==='image_url') {if(!part.image_url.url.startsWith('data:image/'))throw Error('missing image content');evidence.images++;}
    }
    const textOf=m=>typeof m.content==='string'?m.content:(m.content||[]).filter(c=>c.type==='text').map(c=>c.text).join('\n');
    const user=messages.findLastIndex(m=>m.role==='user');
    const results=messages.slice(user+1).filter(m=>m.role==='tool');
    const tools=body.tools||[];
    const marker=/MVP_[a-f0-9]{12}/.exec(textOf(messages[user]||{content:''}))?.[0];
    let tool,args,content;
    // Session-title generation shares the same HTTP provider, without tools.
    if(!tools.length)content='MVP file verification';
    else {
      if(!marker)throw Error('missing journey marker');
      const upload=/verbatim read-only copy saved at ("(?:[^"\\]|\\.)*")/.exec(messages.map(textOf).join('\n'));
      if(!upload)throw Error('DSH did not retain an uploaded file handle');
      if(!results.length) {tool='write';args={file_path:`${marker}.txt`,content:marker};}
      else if(results.length===1) {
        if(!JSON.stringify(results[0]).includes(`${marker}.txt`))throw Error('missing actual write result');
        evidence.writeResults++;tool='read';args={file_path:`${marker}.txt`};
      } else if(results.length===2) {
        if(!textOf(results[1]).includes(marker))throw Error('missing actual workspace read contents');
        evidence.readResults++;tool='read';args={file_path:JSON.parse(upload[1])};
      } else {
        if(!/Uploaded MVP_[a-f0-9]{12}/.test(textOf(results.at(-1))))throw Error('missing actual attachment read contents');
        evidence.attachmentReads++;content=`Verified ${marker}: workspace and uploaded file read by DSH tools.`;
      }
      if(tool&&!tools.some(t=>t.function?.name===tool))throw Error('DSH did not advertise required tool');
    }
    if(body.stream===false)return json({id:'mvp-completion',object:'chat.completion',model:body.model,choices:[{index:0,message:{role:'assistant',content},finish_reason:'stop'}]});
    res.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-cache'});
    const frame=(delta,finish=null)=>res.write(`data: ${JSON.stringify({id:'mvp-completion',object:'chat.completion.chunk',model:body.model,choices:[{index:0,delta,finish_reason:finish}]})}\n\n`);
    frame({role:'assistant',content:''});
    if(tool) {
      frame({tool_calls:[{index:0,id:`call_${marker}_${results.length}`,type:'function',function:{name:tool,arguments:JSON.stringify(args)}}]});frame({},'tool_calls');
    } else {
      for(const word of content.split(' ')) {frame({content:word+' '});await new Promise(resolve=>setTimeout(resolve,200));}frame({},'stop');
    }
    res.end('data: [DONE]\n\n');
  } catch(error) {console.error(error.message);if(!res.headersSent)res.writeHead(500);res.end(JSON.stringify({error:{message:error.message}}));}
}).listen(18080,'0.0.0.0');
