// 轻量 Markdown 渲染（无依赖，Chrome 109 安全）：标题/加粗/斜体/行内码/有序无序列表/引用/分隔线/段落。
// 原则：先整体 HTML 转义再变换，杜绝注入；围栏代码块由 AssistantMsg 的 CodeBlock 处理，不在此处。
function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function inline(t) {
  return t
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
}

export function mdToHtml(src) {
  const lines = esc(src || '').split('\n');
  let html = '';
  let list = null;
  const closeList = () => {
    if (list) {
      html += `</${list}>`;
      list = null;
    }
  };
  for (const l of lines) {
    const h = l.match(/^(#{1,4})\s+(.*)$/);
    if (h) {
      closeList();
      const lv = h[1].length + 1;
      html += `<h${lv}>${inline(h[2])}</h${lv}>`;
      continue;
    }
    if (/^\s*[-*+]\s+/.test(l)) {
      if (list !== 'ul') {
        closeList();
        html += '<ul>';
        list = 'ul';
      }
      html += `<li>${inline(l.replace(/^\s*[-*+]\s+/, ''))}</li>`;
      continue;
    }
    if (/^\s*\d+[.)]\s+/.test(l)) {
      if (list !== 'ol') {
        closeList();
        html += '<ol>';
        list = 'ol';
      }
      html += `<li>${inline(l.replace(/^\s*\d+[.)]\s+/, ''))}</li>`;
      continue;
    }
    if (/^\s*(?:-{3,}|\*{3,})\s*$/.test(l)) {
      closeList();
      html += '<hr />';
      continue;
    }
    if (/^\s*>\s?/.test(l)) {
      closeList();
      html += `<blockquote>${inline(l.replace(/^\s*>\s?/, ''))}</blockquote>`;
      continue;
    }
    if (!l.trim()) {
      closeList();
      continue;
    }
    closeList();
    html += `<p>${inline(l)}</p>`;
  }
  closeList();
  return html;
}
