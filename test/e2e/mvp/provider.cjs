// Deterministic HTTP model boundary. DSH executes every requested tool itself.
const http = require('node:http');
const evidence = {requests:0,writeResults:0,readResults:0,images:0};
http.createServer(async (req,res) => {
  if(req.url==='/evidence') {res.setHeader('content-type','application/json');res.end(JSON.stringify(evidence));return;}
  if(req.url.endsWith('/models')) {res.setHeader('content-type','application/json');res.end(JSON.stringify({object:'list',data:[{id:'deepseek-v4-flash',object:'model'}]}));return;}
  if(!req.url.endsWith('/chat/completions')) {res.writeHead(404);res.end();return;}
  try {
    let raw='';for await(const chunk of req) {raw+=chunk;if(raw.length>8*1024*1024)throw Error('request too large');}
    const body=JSON.parse(raw);evidence.requests++;
    const messages=body.messages||[];
    evidence.images+=messages.filter(m=>Array.isArray(m.content)&&m.content.some(c=>c.type==='image_url')).length;
    const user=messages.findLastIndex(m=>m.role==='user');
    const results=messages.slice(user+1).filter(m=>m.role==='tool');
    const tools=body.tools||[];
    const named=name=>tools.find(t=>t.function?.name===name)?.function.name;
    const prompt=JSON.stringify(messages[user]?.content||'');
    const marker=/MVP_[a-f0-9]{12}/.exec(prompt)?.[0];
    if(!marker)throw Error('missing journey marker');
    let tool, args, content;
    if(!results.length) {tool=named('write');args={file_path:`${marker}.txt`,content:marker};}
    else if(results.length===1) {
      if(!JSON.stringify(results[0]).includes(`${marker}.txt`))throw Error('missing actual write result');
      evidence.writeResults++;tool=named('read');args={file_path:`${marker}.txt`};
    } else {
      if(!JSON.stringify(results.at(-1)).includes(marker))throw Error('missing actual read contents');
      evidence.readResults++;content=`Verified ${marker}: file written and read by DSH tools.`;
    }
    if(!content&&!tool)throw Error('DSH did not advertise the required file tool');
    res.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-cache'});
    const frame=(delta,finish=null)=>res.write(`data: ${JSON.stringify({id:'mvp-completion',object:'chat.completion.chunk',model:body.model,choices:[{index:0,delta,finish_reason:finish}]})}\n\n`);
    frame({role:'assistant',content:''});
    if(tool) {
      frame({tool_calls:[{index:0,id:`call_${marker}_${results.length}`,type:'function',function:{name:tool,arguments:JSON.stringify(args)}}]});
      frame({},'tool_calls');
    } else {
      for(const word of content.split(' ')) {frame({content:word+' '});await new Promise(resolve=>setTimeout(resolve,100));}
      frame({},'stop');
    }
    res.end('data: [DONE]\n\n');
  } catch(error) {if(!res.headersSent)res.writeHead(500);res.end(JSON.stringify({error:{message:error.message}}));}
}).listen(18080,'0.0.0.0');
