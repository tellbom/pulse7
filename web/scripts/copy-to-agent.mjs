// 将构建产物复制到 agent/web（pulse7 serve 嵌入目录），随后可用 go build 打包进 exe。
import { cpSync, rmSync, mkdirSync, existsSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = resolve(here, '../dist');
const target = resolve(here, '../../agent/web');

if (!existsSync(dist)) {
  console.error('先运行 npm run build 生成 dist/');
  process.exit(1);
}
rmSync(target, { recursive: true, force: true });
mkdirSync(target, { recursive: true });
cpSync(dist, target, { recursive: true });
console.log('已复制 dist/ → agent/web/（pulse7 serve 将以此提供页面并注入 token）');
