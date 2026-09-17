// Markdown 渲染：markdown-it（CommonMark + GFM 表格/删除线），Chrome 109 可用。
// html:false 让模型输出里的原始 HTML 按文本转义，不需要额外的 sanitizer；
// markdown-it 默认拒绝 javascript:/vbscript:/data: 链接。
// breaks:true 保持此前"每行一段"的观感（正文容器不再依赖 pre-wrap）。
// 围栏代码块由 AssistantMsg 先切给 CodeBlock，不经过这里。
import MarkdownIt from 'markdown-it';

const md = new MarkdownIt({ html: false, linkify: false, breaks: true, typographer: false });

// 外链在新窗口打开，与旧渲染器行为一致。
const defaultLinkOpen =
  md.renderer.rules.link_open || ((tokens, idx, options, env, self) => self.renderToken(tokens, idx, options));
md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('target', '_blank');
  tokens[idx].attrSet('rel', 'noopener');
  return defaultLinkOpen(tokens, idx, options, env, self);
};

export function mdToHtml(src) {
  return md.render(String(src || ''));
}
