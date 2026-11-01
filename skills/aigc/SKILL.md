# 虾盘云创作（作图 + 音乐）

直接调用虾盘云的 AI 作图与音乐生成。脚本是零 npm 依赖的 Node 程序，已随虾盘内置；API Key 由虾盘通过环境变量 `XIAPAN_API_KEY` 注入，**不要**在命令里写 key。

## 什么时候用

- 用户要「画图 / 作图 / 文生图 / 图生图 / 生成图片 / 配图 / 海报背景」→ `gen-image.mjs`
- 用户要「一首歌 / 带人声 / 歌词 / 音乐 / 作曲」→ `gen-music.mjs`
- 用户要「纯音乐 / 背景配乐 / BGM / 无人声配乐」→ `gen-bgm.mjs`

## 先检查 node，没有就明确提示

```bash
node --version
```

如果返回「找不到 node / command not found」，**直接告诉用户**：「这台电脑没有可用的 Node 运行时，作图/音乐功能暂不可用；你可以用文字对话或电脑维护工具。」不要假装成功，也不要反复重试。

## 作图：skills/aigc/scripts/gen-image.mjs

先把用户一句话补成专业提示词（主体 + 细节/材质 + 风格 + 光影 + 构图 + 画质词），再按用途选尺寸。

```bash
# 文生图
node skills/aigc/scripts/gen-image.mjs --prompt "补全后的专业描述" --model gpt-image-2 --size 1024x1024 --out out.png --json

# 图生图（--ref 可重复多张做融合）
node skills/aigc/scripts/gen-image.mjs --prompt "把背景换成星空" --ref input.png --out out2.png --json
```

尺寸（按用途选，只填 `宽x高` 像素，别填比例文字）：
- 抖音/短视频封面 `1536x2048`；小红书/手机壁纸 `1024x1536`；头像/图标/白底图 `1024x1024`；横图/banner/文章头图 `1536x1024`。

风格短语（拼进 `--prompt`）：写实 `photorealistic, cinematic lighting, 8k, ultra detailed`；扁平插画 `flat illustration, minimal, vector style`；3D `3D render, octane render, soft studio light`；中国风 `Chinese ink painting style, guochao`；二次元 `anime style, cel shading`。

**别硬做**：图里要准确中文长句/二维码/精确 LOGO 文字，扩散模型做不准，先出背景图再让用户另叠字。

`--json` 输出的 `file` 字段是落盘图片绝对路径；成功退出码 0，失败退出码 1/2。出图可能耗时 1~4 分钟，属于正常。

## 音乐：gen-music.mjs / gen-bgm.mjs

```bash
# 带人声的整首歌（Suno，一次出 2 首）
node skills/aigc/scripts/gen-music.mjs --prompt "一首关于夏天海边的轻快流行歌，男声" --out song --json

# 纯音乐 / 配乐（MiniMax）
node skills/aigc/scripts/gen-bgm.mjs --prompt "深夜咖啡馆的爵士钢琴，慵懒治愈" --out bgm.mp3
```

音乐是异步：提交 → 轮询 → 下载，整首歌可能几分钟；脚本内部已做断点续跑，不要重复提交。成功输出 `<前缀>-1.mp3`、`<前缀>-2.mp3` 等文件路径。

## 输出路径

`--out` 传相对路径会落在当前工作目录（虾盘会以项目目录为工作目录）。建议把 `--out` 写成项目目录下的明确路径，方便用户找到。

## 失败怎么回话

- 余额不足 / Invalid token → 告诉用户「虾盘云余额不足或凭证失效，请去设备钱包充值或刷新」。
- 网络错误 / 超时 → 告诉用户「虾盘云连接失败，请检查网络后重试」。
- 其余错误 → 原样转述脚本 stderr 的中文错误，不要编造成因。
