# Aozora

日本のテレビ番組の見逃し配信を見るための、デスクトップクライアントです。
[MyGo](https://mygo.egoist.dev)(Go + WebView)で作っています。UIは Material Design 3 です。

> **非公式・個人利用向けです。** TVer および各放送局とは関係ありません。
> TVer の公開されていないAPIを利用しているため、仕様変更で突然動かなくなることがあります。
> 利用は、TVer の利用規約の範囲で、自己責任でお願いします。

## できること

- ホーム、カテゴリ別ブラウズ、ランキング(番組・エピソード × ジャンル別、50位まで)
- 検索(番組とエピソードを分けて表示)、シリーズのシーズン別表示
- 独自のプレイヤー: シーク、10秒スキップ、音量、再生速度、ピクチャーインピクチャー、ミニプレイヤー、ウィンドウ全画面
- 再生準備の進行表示(取得経路、マニフェスト読み込みなど)
- 設定画面から [yt-dlp](https://github.com/yt-dlp/yt-dlp) を公式リリースからダウンロード・更新(SHA-256 を検証)

## 動作の仕組み

- 再生URLは、まず `yt-dlp` で取得し、失敗したときだけ Brightcove から直接取得します。
  `yt-dlp` は、設定画面で入れたアプリ内のコピーを優先し、なければ `PATH` 上のものを使います。
- 配信元は TVer 以外のオリジンからのリクエストを拒否するため、映像は `127.0.0.1` のローカルプロキシ経由で再生します。

## 必要なもの

- Go 1.27 以上
- [Bun](https://bun.sh)
- Windows 10 では WebView2 ランタイム(Windows 11 には含まれます)
- Linux では GTK 3 と WebKitGTK 4.1

`bunx mygo doctor` で環境を確認できます。

## 開発とビルド

```sh
bun install
bun run dev      # ライブリロードつきで起動
bun run build    # build/<os>-<arch>/ にパッケージを作成
```

## 構成

| ファイル | 役割 |
|---|---|
| `main.go` | ウィンドウの作成 |
| `tver.go` | TVer のAPI(ホーム、検索、シリーズ、再生URL) |
| `ranking.go` | ランキング |
| `ytdlp.go`, `ytdlp_manage.go` | yt-dlp での再生URL取得と、ダウンロード・更新 |
| `proxy.go` | 配信元に TVer のオリジンを付けるローカルプロキシ |
| `window.go` | ウィンドウの全画面切り替え |
| `src/` | フロントエンド(TypeScript)。`src/mygo.ts` は `mygo generate` が生成 |

## リリース

`v*` のタグを push すると、GitHub Actions(`.github/workflows/release.yml`)が Windows 用(インストーラと実行ファイル)と macOS 用(`.dmg`、Apple Silicon と Intel の両対応)をビルドして、GitHub のリリースに添付します。
`main` への push とプルリクエストでは、ビルドの確認だけを行います(リリースは作りません)。

```sh
# 1. mygo.config.ts の version を上げてコミット
# 2. 同じ番号のタグを作って push(例: version が 0.2.0 なら v0.2.0)
git tag v0.2.0
git push origin v0.2.0
```

タグと `mygo.config.ts` の version が違うと、ビルドは失敗します。
署名と公証はしていないので、初回起動時に Windows の SmartScreen や macOS の Gatekeeper の警告が出ます。
