import { createApp } from 'vue';
import './style.css';
const token = document.querySelector('meta[name=pulse7-token]').content;
createApp({
 data: () => ({ config: {}, key: '', workspace: '', prompt: '', sessionId: '', sessions: [], history: [], hasMore: false, events: [], attempts: {}, permission: null, connected: false, status: '连接中', error: '', profile: 'open', listener: {}, tasks: [], processes: [], overview: {}, checkpoints: [], target: '', results: {}, outputs: {}, context: null }),
 async mounted() {
  await this.act(async () => {
   this.config = await this.api('GET', '/api/config'); this.workspace = this.config.workspace;
   this.listener = await this.api('GET', '/api/listener');
   await this.refresh(); this.stream();
  });
 },
 methods: {
  async api(method, path, body) {
   const r = await fetch(path, { method, headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
   const data = await r.json(); if (!r.ok) throw new Error(data.error?.message || `HTTP ${r.status}`); return data;
  },
  async act(fn) { this.error = ''; try { await fn(); } catch (e) { this.error = e.message; } },
  async save() { await this.act(async () => { const c = { base_url: this.config.base_url, model: this.config.model }; if (this.key) c.api_key = this.key; this.config = await this.api('PUT', '/api/config', c); this.key = ''; this.status = '已保存到用户全局配置'; }); },
  async testConnection() { await this.act(async () => { this.status = '连接测试中'; const c = { base_url: this.config.base_url, model: this.config.model }; if (this.key) c.api_key = this.key; const r = await this.api('POST', '/api/connection-test', c); this.status = `连接成功 ${r.elapsedMs}ms（未隐式保存）`; }); },
  async selectWorkspace() { await this.act(async () => { const r = await this.api('PUT', '/api/workspace', { path: this.workspace }); this.workspace = r.workspace; this.sessionId = ''; this.history = []; this.attempts = {}; this.status = '工作区已选择'; await this.refresh(); }); },
  async refresh() { const [tasks, sessions, permissions] = await Promise.all([this.api('GET', '/api/tasks'), this.api('GET', '/api/sessions'), this.api('GET', '/api/permissions')]); this.tasks = tasks.tasks; this.processes = tasks.processes; this.overview = tasks.overview; this.sessions = sessions.sessions; this.profile = permissions.profile; },
  async start(answer = false) { await this.act(async () => { const r = await this.api('POST', answer ? '/api/answer' : '/api/turns', answer ? { sessionId: this.sessionId, answer: this.prompt } : { prompt: this.prompt }); this.sessionId = r.sessionId; this.status = '等待模型响应'; }); },
  async interrupt() { await this.act(async () => { this.status = '已请求中断，等待结束事件'; await this.api('POST', '/api/interrupt', {}); this.permission = null; }); },
  async confirm(decision) { await this.act(async () => { const requestId = this.permission.requestId; await this.api('POST', '/api/permission', { requestId, decision }); if (this.permission?.requestId === requestId) this.permission = null; }); },
  async setProfile() { await this.act(async () => { await this.api('PUT', '/api/permissions', { profile: this.profile }); }); },
  async resume(id) { await this.act(async () => { const r = await this.api('POST', '/api/sessions/resume', { sessionId: id }); this.sessionId = r.sessionId; this.history = []; this.attempts = {}; await this.moreHistory(0); this.status = '历史已恢复，可继续发送任务'; }); },
  async moreHistory(offset = this.history.length) { const r = await this.api('GET', `/api/sessions/${encodeURIComponent(this.sessionId)}/messages?offset=${offset}&limit=100`); this.history.push(...r.messages); this.hasMore = r.hasMore; },
  async expand(e) { await this.act(async () => { this.results[e.id] = (await this.api('GET', `/api/tool-result?ref=${encodeURIComponent(e.resultRef)}`)).result; }); },
  async checkpointsList() { await this.act(async () => { this.checkpoints = (await this.api('GET', '/api/checkpoints')).checkpoints; }); },
  async rollback() { await this.act(async () => { this.status = (await this.api('POST', '/api/rollback', { to: Number(this.target) })).result; }); },
  async kill(id) { await this.act(async () => { this.status = (await this.api('POST', '/api/tasks/kill', { task_id: id })).result; await this.refresh(); }); },
  async stream() {
   try {
    const r = await fetch('/api/events', { headers: { Authorization: `Bearer ${token}` } }); if (!r.ok) throw new Error(`SSE ${r.status}`);
    this.connected = true; this.status = '就绪'; const reader = r.body.getReader(); const decoder = new TextDecoder(); let buffer = '';
    while (true) {
     const { value, done } = await reader.read(); if (done) throw new Error('事件流已断开；未承诺历史事件回放');
     buffer += decoder.decode(value, { stream: true }); let end;
     while ((end = buffer.indexOf('\n\n')) >= 0) { const frame = buffer.slice(0, end); buffer = buffer.slice(end + 2); const line = frame.split('\n').find(x => x.startsWith('data: ')); if (line) await this.consume(JSON.parse(line.slice(6))); }
    }
   } catch (e) { this.connected = false; this.error = e.message; this.status = '事件流断开'; }
  },
  async consume(event) {
   const d = event.data; this.events.push(event);
   if (event.type === 'session_init') this.sessionId = d.sessionId;
   if (event.type === 'assistant_attempt') { if (d.status === 'start') this.attempts[d.attempt] = { text: '', status: 'streaming' }; if (d.status === 'discard') this.attempts[d.attempt] = { text: '', status: 'discarded' }; if (d.status === 'complete' && this.attempts[d.attempt]) this.attempts[d.attempt].status = 'complete'; }
   if (event.type === 'assistant_delta') { const a = this.attempts[d.attempt]; if (a) a.text += d.delta; }
   if (event.type === 'permission_request') { this.permission = d; this.status = '等待权限确认'; }
   if (event.type === 'permission_response') this.permission = null;
   if (event.type === 'tool_call') this.status = `工具尚未返回：${d.name}`;
   if (event.type === 'heartbeat') this.status = `等待模型首块 ${d.waitedSeconds}s`;
   if (event.type === 'context_state') this.context = d;
   if (event.type === 'background_task') {
    if (d.action === 'output') {
     let offset = d.offset || 0;
     while (offset < d.nextOffset) {
      const out = await this.api('GET', `/api/tasks/${encodeURIComponent(d.taskId)}/output?offset=${offset}&limit=${Math.min(1048576, d.nextOffset - offset)}`);
      if (out.nextOffset <= offset) throw new Error('后台输出未能读取到事件声明的位置');
      this.outputs[d.taskId] = (this.outputs[d.taskId] || '') + out.content;
      offset = out.nextOffset;
     }
    }
    await this.refresh();
   }
   if (event.type === 'process_mode') this.overview.mode = d;
   if (event.type === 'turn_result') { this.permission = null; this.status = d.status + (d.error ? ': ' + d.error : ''); await this.refresh(); }
  }
 },
 template: `<main><header><h1>pulse7 接入验证</h1><p>占位页 · 非正式界面</p><p>{{listener.address}}:{{listener.port}} · {{connected ? 'SSE 已连接' : 'SSE 未连接'}} · token {{listener.tokenValid ? '有效' : '无效'}}</p></header>
 <p role="status">{{status}}</p><p v-if="error" class="error" role="alert">{{error}}</p>
 <div class="columns"><section><fieldset><legend>配置与工作区</legend><label>Endpoint<input v-model="config.base_url"></label><label>Model<input v-model="config.model"></label><label>API key（留空保留）<input type="password" v-model="key" autocomplete="off"></label><button @click="save">保存全局配置</button><button @click="testConnection">连接测试</button><label>工作区<input v-model="workspace"></label><button @click="selectWorkspace">选择工作区</button><label>权限档位<select v-model="profile" @change="setProfile"><option>strict</option><option>standard</option><option>open</option></select></label></fieldset>
 <fieldset><legend>任务 · {{sessionId || '尚无会话'}}</legend><textarea v-model="prompt" rows="4" aria-label="任务文本"></textarea><button :disabled="!connected" @click="start(false)">发送任务</button><button :disabled="!connected || !sessionId" @click="start(true)">回答并继续</button><button @click="interrupt">中断当前轮</button></fieldset>
 <fieldset v-if="permission"><legend>权限确认</legend><pre>{{permission.tool}} {{permission.target}}\n{{JSON.stringify(permission.args,null,2)}}</pre><button @click="confirm('allow')">允许</button><button @click="confirm('deny')">拒绝 / 取消</button></fieldset>
 <h2>历史会话</h2><button @click="act(refresh)">刷新列表</button><div v-for="s in sessions" :key="s.sessionId"><button @click="resume(s.sessionId)">{{s.sessionId}}</button> {{s.firstUser}}</div><article v-for="(m,i) in history" :key="i"><strong>{{m.role}}</strong><pre>{{m.content}}</pre><pre v-if="m.tool_calls">{{JSON.stringify(m.tool_calls,null,2)}}</pre></article><button v-if="hasMore" @click="act(()=>moreHistory())">更多历史</button>
 <h2>本轮输出</h2><article v-for="(a,id) in attempts" :key="id"><small>尝试 {{id}} · {{a.status}}</small><pre>{{a.text}}</pre></article>
 <details v-for="(e,i) in events.filter(x=>x.type!=='assistant_delta')" :key="i"><summary>{{e.type}} {{e.data.summary || e.data.action || ''}}</summary><pre>{{JSON.stringify(e.data,null,2)}}</pre><button v-if="e.type==='tool_result'" @click="expand(e.data)">展开完整结果</button><pre v-if="results[e.data.id]">{{results[e.data.id]}}</pre></details></section>
 <aside><h2>后台任务与进程</h2><p>仅会话 Job 成员保证清理；第三方后代可脱离。</p><pre>{{JSON.stringify(overview,null,2)}}</pre><p v-if="context">上下文估算 {{context.usedTokens}} / {{context.budget}}，剩余 {{context.percentLeft.toFixed(1)}}%</p><article v-for="t in tasks" :key="t.taskId"><strong>{{t.taskId}} · {{t.status}}</strong><p>{{t.command}}</p><p v-if="t.detached">将在 pulse7 退出后继续运行</p><button @click="kill(t.taskId)">终止任务</button><pre>{{outputs[t.taskId]}}</pre></article><details><summary>登记进程</summary><pre>{{JSON.stringify(processes,null,2)}}</pre></details><h2>Checkpoint</h2><button @click="checkpointsList">列出 checkpoint</button><select v-model="target"><option value="">选择回滚目标</option><option v-for="c in checkpoints" :value="c.seq">{{c.seq}} · {{c.kind}} · {{c.createdAt}} · {{c.commit}}</option></select><p v-for="c in checkpoints.filter(x=>x.seq==target)">工作区相对 HEAD 的变更文件数：{{c.dirtyFiles===null?'未知':c.dirtyFiles}}；{{c.dirtyFilesBasis}}</p><button :disabled="!target" @click="rollback">回滚到所选目标</button></aside></div></main>`
}).mount('#app');
