<p align="center">
  <img src="banner.svg" alt="U-PicoClaw — portable AI with offline maintenance tools" width="100%">
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a></p>

# U-PicoClaw · Xiapan USB

**軽量な AI インターフェース、持ち運べるウォレットと API キー、PC 上のプログラムキャッシュ、オフラインのメンテナンスツール。**

![Preview](https://img.shields.io/badge/version-0.5.4--portable--preview-8b6b33)
![Go](https://img.shields.io/badge/Go-1.24%2B-367a8d)
![License](https://img.shields.io/badge/license-MIT-527c46)
![AI optional](https://img.shields.io/badge/AI-optional-4c7770)

[クイックスタート](#クイックスタート) · [対応プラットフォーム](#対応プラットフォーム) · [USB パッケージの作成](#usb-パッケージの作成) · [貢献ガイド](CONTRIBUTING.md)

> [!NOTE]
> **現在はプレビュー版です。** パッケージの作成手順は Windows x64、macOS Apple Silicon と Intel に対応します。Apple Silicon は ExFAT の実物 USB で起動、ファイル操作、認証情報変更後の再起動と終了を検証済みです。Intel と Finder の初回許可は未検証です。Windows のツールが Mac でも使えるわけではありません。[製品範囲（中国語）](docs/PRODUCT.md)も確認してください。

## AI と応急ツールを持ち歩く

最初は従来の軽量インターフェースを開きます。複雑な作業、スキル、チャネル、詳細設定には同じ画面の **Full PicoClaw** から公式ワークベンチを起動します。両方の画面で同じ常駐 Gateway を使用します。ページを閉じても停止しないため、終了時は **Quit** を選び、USB を安全に取り外してください。

| 機能 | 内容 |
| --- | --- |
| ウォレットと API キー | Xiapan Cloud または自分の互換 API を利用。設定と会話は USB に保存し、ホーム画面を開くだけではウォレットを作成しません。 |
| 必要なときにチャージ | 残高を確認して支払いページを開けます。支払いは利用者が実行します。 |
| PC 上のプログラムキャッシュ | 検証済み PicoClaw 実行ファイルを PC にキャッシュし、USB の繰り返し読み出しを減らします。キー、ウォレット、会話、作業ファイルは USB に残します。 |
| オフラインの応急ツール | モデル、API キー、クラウド残高なしで PC を診断し、対応するツールを開けます。 |
| UI 言語 | 中国語と英語を切り替えられ、選択は USB に保存されます。日本語 UI は未提供です。 |

初回のキャッシュ作成には USB の読み出しが必要です。ホーム画面、Gateway、モデルの初回応答は別々に測定します。**0.3 秒起動は検証済みの保証ではありません。** 容量はプラットフォームとツール構成で変わります。ローカルモデルの重み・推論ランタイムや OpenClaw は同梱しません。

## クイックスタート

**Git と Go 1.24 以降**を用意してください。各 OS で同じリポジトリを使用します。

```sh
git clone --branch codex/u-picoclaw-portable https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
```

Windows では `go build -o dist/xiapan.exe .` と `./dist/xiapan.exe` を使用します。この手順でビルドされるのはコアプログラムです。PicoClaw と外部ツールは自動では同梱されません。[開発ガイド（中国語）](docs/DEVELOPMENT.md)も参照してください。

## ツールカタログ

Windows の作成手順には **ポータブルツール 9 種と標準ツールへの入口 9 種**が含まれます。追加ダウンロードは状態を明示します。カタログの項目数は、すべての OS で全ツールを利用できるという意味ではありません。

| 対応済みツール | 用途 |
| --- | --- |
| WinDirStat | ディスク使用量の可視化 |
| PeaZip | ファイルの圧縮と展開 |
| Everything | ファイル検索 |
| Explorer++ | ファイルの閲覧と管理 |
| Notepad++ | テキストと設定ファイルの編集 |
| SumatraPDF | PDF の閲覧 |
| CrystalDiskInfo | ディスクの状態と SMART 情報の確認 |
| PhotoRec | 別の正常な保存先へのファイル復旧 |
| TestDisk | パーティションと復旧手段の確認 |

バージョン、ファイル、SHA-256、ライセンスを固定マニフェストに記録し、許可したファイルだけをパッケージにコピーします。ハードウェア識別とドライバーの準備機能はありますが、ドライバーの自動照合・インストール・ロールバックは未実装です。

Mac には Activity Monitor、Disk Utility、Console、System Information、Wireless Diagnostics の 5 つの OS 標準ツールへの入口があります。Activity Monitor は実機で起動を確認し、残りの 4 つはシステム上のアプリのパスを確認済みです。[English quick start](docs/QUICKSTART.en.md) / [中文快速入门](docs/QUICKSTART.zh-CN.md) に USB へのコピー手順と API 設定を記載しています。

## PicoClaw は一つ、画面は二つ

| 入口 | 用途 |
| --- | --- |
| 軽量ホーム画面 | 会話、モデル設定、ウォレット、メンテナンス。 |
| Full PicoClaw | 必要なときに起動する公式ワークベンチと詳細設定。 |
| メンテナンスガイド | AI やクラウド残高を使わない診断とツール起動。 |

コアと公式 launcher は **PicoClaw v0.3.1** に固定し、配布用パッチで自己更新を無効にします。両方の実行ファイルを SHA-256 で固定し、利用者の USB が latest/nightly を自動追跡することはありません。

PicoClaw のツールは現在の OS ユーザーの権限で動作します。GUI・CLI・メンテナンス MCP は同じアクションコアを利用します。メンテナンス操作の確認規則は、PicoClaw が実行できる任意のシェルコマンドを制限するものではありません。

![GUI・CLI・AI が共有するメンテナンスコア](diagrams/action-core.svg)

## 対応プラットフォーム

| プラットフォーム | 検証範囲 |
| --- | --- |
| **Windows x64** | 従来のメンテナンスフローは実物 USB で検証済みです。0.5 の Gateway・キャッシュ機能は今回のリリース検証記録で判断してください。 |
| macOS · Apple Silicon / Intel | Apple Silicon は ExFAT の実物 USB で起動、ファイル操作、認証情報変更後の再起動と終了を検証済みです。Intel、Finder の初回許可と別 PC への移動は未検証です。 |
| Linux x64 | コアのソースとビルド対象として維持しますが、今回の製品パッケージには含めません。従来の WSL テストは Linux デスクトップ版の出荷検証ではありません。 |

同じドライブに複数 OS 用のプログラムを保存できますが、各 OS 専用の実行ファイルが必要です。**現時点では起動可能なレスキュー OS ではありません。** スタートアップ検査は一般的な Run 項目とスタートアップフォルダーを対象とし、全サービスやタスクを網羅せず、自動無効化もしません。

## USB パッケージの作成

保守担当者には Go、PowerShell、Python 3.11 以降が必要です。公式ワークベンチの再ビルドには Node.js と pnpm も必要です。利用者にこれらのビルドツールは不要です。

```powershell
python scripts/build-picoclaw-pinned.py
.\scripts\build.ps1
python scripts/package-portable.py --resources "D:\verified-tool-resources" --output "D:\release\U-PicoClaw-0.5.4-portable-preview"
```

リソース元には固定マニフェストのツールファイルを用意し、出力にはまだ存在しないディレクトリを指定します。個人データをコピーせず、空の data と検証用ハッシュを生成します。新しいパッケージにはこの手順を使用してください。詳細は[開発ガイド（中国語）](docs/DEVELOPMENT.md)を参照してください。

```text
虾盘.exe   Windows 起動プログラム
app/       OS 別プログラム・固定 PicoClaw・ツール・ライセンス
data/      設定・ウォレット・会話・作業ファイル
drivers/   準備済みドライバーリソース
```

> [!IMPORTANT]
> API キーとバックアップは現在、ドライブ内に**平文で保存**されます。ドライブを適切に管理してください。別の PC でも接続可能な API と有効なキーが必要です。localhost のモデルサーバーは利用先の PC で起動してください。利用者の USB は自己更新しません。将来の検証済みリリースへ移行するときも `data/` と各ツールの個人設定を保持し、使用済みのデータを出荷用にコピーしないでください。

## オープンソースと設定済みドライブ

本プロジェクト独自のソースは [MIT ライセンス](LICENSE)です。無料で入手して自分で準備することも、提供される場合は設定済みドライブを購入することもできます。物理ドライブの価値は、ハードウェア、事前ダウンロード、設定・動作確認、サポートです。ローカルの基本機能にクラウドへの課金は必要ありません。

外部ソフトウェアには各ライセンスが適用されます。[第三者ライセンス](THIRD_PARTY.md)と[配布ガイド（中国語）](docs/DISTRIBUTION.md)を確認してください。

<details>
<summary>プロジェクトのコンセプトアート</summary>

![Xiapan USB Toolkit のコンセプトイラスト](assets/xiapan-usb-concept.png)

出荷製品の写真ではなく、プロジェクト用のイラストです。[デザインと使用ツール](docs/README-DESIGN.md)。

</details>

## 開発に参加する

プラットフォーム対応、再現可能な不具合報告、ポータブルツールの追加を歓迎します。[貢献ガイド](CONTRIBUTING.md)、[セキュリティ](SECURITY.md)、[Issue](https://github.com/dongsheng123132/xiapan-usb/issues)をご利用ください。ウォレットの URL、API キー、個人の診断データは公開しないでください。アプリ UI は中国語と英語に対応し、この日本語 README は日本語 UI の提供を意味しません。

[Xiapan Cloud](https://cloud.u-claw.org/) · [U-King](https://www.u-king.org/) · [Xiapan Canvas](https://tu.u-claw.org.cn/)
