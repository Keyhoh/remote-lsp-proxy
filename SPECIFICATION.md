# Remote LSP Proxy 設計書

## 1. 概要

### 1.1. 目的

ローカルのエディタから、リモート環境で動作するLanguage Server Protocol（LSP）サーバーを利用するためのProxyを実装する。

LSPクライアントとLanguage Serverの間にProxyを配置し、通信の中継と必要なメッセージ変換を行う。

初期の実行環境としてDev Containerを想定するが、設計上はDev Containerに依存しない汎用的なRemote LSP Proxyとする。

### 1.2. 仕様準拠

LSPの仕様は以下を基準とする。

- LSP 3.18
- JSON-RPC 2.0

仕様：
https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/

LSPの仕様に定義されているメッセージ構造とデータ型を基準に、変換対象を管理する。

### 1.3. 想定構成

```text
┌──────────────────────────────┐
│         Local Host           │
│                              │
│  ┌────────────┐              │
│  │   Neovim   │              │
│  └─────┬──────┘              │
│        │ stdio               │
│  ┌─────▼──────┐              │
│  │   Proxy    │              │
│  └─────┬──────┘              │
│        │                     │
│  ┌─────▼──────┐              │
│  │  Backend   │              │
│  └─────┬──────┘              │
└────────┼─────────────────────┘
         │ Remote Execution
         │
┌────────▼─────────────────────┐
│       Remote Environment     │
│                              │
│  ┌────────────┐              │
│  │ Language   │              │
│  │  Server    │              │
│  └────────────┘              │
│                              │
└──────────────────────────────┘
```

Proxyはローカルで起動し、Remote Backendを通じてリモート環境のLanguage Serverを起動する。

初期実装ではDev Container CLIの`exec`を利用する。

```text
Neovim
  │
  │ stdio
  ▼
Remote LSP Proxy
  │
  │ devcontainer exec
  ▼
Language Server
```

ProxyとLanguage Server間の通信は標準入出力を利用する。リモート環境に専用のTCPサーバーや常駐プロセスを追加する必要はない。

### 1.4. 基本方針

- LSPプロトコル自体は変更しない。
- 必要なメッセージだけを解析・変換し、それ以外は透過的に中継する。
- LSP 3.18の仕様に基づいてURI変換対象を管理する。
- JSON-RPCの双方向通信を前提とする。
- リモート実行環境への依存をBackendに閉じ込める。
- ファイル同期やファイルシステムの代理アクセスは行わない。
- ローカルとリモートで同じ実体のファイルにアクセスできることを前提とする。
- ローカルとリモートでファイルの絶対パスが異なることは許容する。

---

## 2. 前提条件

### 2.1. ファイル共有

ローカルとリモートは同じファイル実体を参照できるものとする。

ただし、ファイルの絶対パスは異なっていてもよい。

例：

| 環境 | Workspace |
|---|---|
| Local | `/home/user/project` |
| Remote | `/workspace/project` |

この場合、ローカルの`/home/user/project/src/main.ts`とリモートの`/workspace/project/src/main.ts`は同じファイルを指す。

Proxyはこのパスの違いを吸収する。

### 2.2. 対象外

初期実装では以下を対象外とする。

- ファイルの同期
- リモートファイルシステムの代理アクセス
- ファイル内容の転送
- ローカルとリモートのファイル内容の整合性管理

### 2.3. LSP通信

LSPはJSON-RPC 2.0をベースとし、標準入出力上でメッセージを送受信する。

JSON-RPCには以下のメッセージ種別がある。

- Request
- Response
- Notification

RequestとNotificationはClientとServerの双方から送信される可能性がある。

したがって、ProxyはClientからServerへの通信だけでなく、ServerからClientへの通信も同等に扱える構造とする。

---

## 3. ローカル実行とリモート実行の差分

Proxyが吸収する必要のある差分を以下に整理する。

| 項目 | ローカル実行 | リモート実行 |
|---|---|---|
| プロセス実行 | ローカル | リモート |
| ファイルアクセス | ローカルFS | リモートFS |
| ファイルパス | ローカルパス | リモートパス |
| Working Directory | ローカルWorkspace | リモートWorkspace |
| 環境変数 | ローカル環境 | リモート環境 |
| プロセス管理 | ローカルOS | リモート実行環境 |
| 通信 | 直接stdio | Backend経由のstdio |
| ファイル監視 | ローカルFSを監視 | リモートFSとの整合性が必要 |

LSPプロトコル自体は共通であるため、基本的には通信経路と実行環境の差分を吸収すればよい。

---

## 4. 機能要件

### 4.1. 必須要件

#### 4.1.1. リモートプロセスの起動・終了管理

リモート環境でLanguage Serverを起動し、そのプロセスを管理する。

必要な機能：

- リモートコマンドの起動
- コマンドライン引数の指定
- リモートWorking Directoryの指定
- 必要な環境変数の指定
- 標準入力・標準出力・標準エラーの取得
- プロセス終了の検知
- プロセス終了時の後処理

Language ServerのプロセスはLSPクライアントごとに起動する。

一方、Dev Containerなどの実行環境自体は複数のLanguage Serverで共有できるものとする。

コンテナのライフサイクルとLanguage Serverのライフサイクルは分離する。

#### 4.1.2. stdioの双方向中継

ローカルのLSPクライアントとリモートのLanguage Server間で標準入出力を双方向に中継する。

```text
Local Client stdin
       │
       ▼
    Proxy
       │
       ▼
Remote Server stdin


Remote Server stdout
       │
       ▼
    Proxy
       │
       ▼
Local Client stdout
```

標準エラー出力はLSP通信と混在させず、別経路で扱う。

必要な仕様：

- 双方向の通信
- EOFの伝播
- プロセス終了の伝播
- バッファリング
- Backpressureへの対応
- 通信中断時の後処理

Proxyの標準出力にはLSP通信以外のログを出力してはならない。

#### 4.1.3. JSON-RPCフレーミング処理

LSPメッセージは以下の形式で送受信される。

```text
Content-Length: 123\r\n
\r\n
{"jsonrpc":"2.0","method":"..."}
```

ProxyはJSON-RPCメッセージを解析するため、フレーミング処理を実装する。

必要な機能：

- `Content-Length`ヘッダーの解析
- メッセージ本文の読み取り
- メッセージ境界の認識
- メッセージ本文のシリアライズ
- 再送信時の`Content-Length`の再計算

ストリームの読み取り単位とメッセージ境界が一致することを前提としてはならない。

一度に複数のメッセージが届く場合や、メッセージが複数回の読み取りに分割される場合にも対応する。

#### 4.1.4. File URIの双方向変換

ローカルとリモートでファイルパスが異なる場合、LSPメッセージに含まれるFile URIを変換する。

変換方向：

- Local URI → Remote URI
- Remote URI → Local URI

例：

```text
Local:
file:///home/user/project/src/main.ts

Remote:
file:///workspace/project/src/main.ts
```

変換は単純な文字列置換ではなく、Workspace Mappingに基づいて行う。

URIのエンコード・デコードやOSごとのパス表現を考慮する。

基本方針：

- `file:` URIのみをパスマッピングの対象とする。
- `http:`や`https:`などのURIは変換しない。
- `untitled:`などの非File URIも変換しない。
- Workspace Mappingの対象外のパスは原則として変換しない。
- パスの前方一致だけでなく、ディレクトリ境界を考慮する。
- URIの正規化を適切に行い、意図しないパスマッピングを防止する。

#### 4.1.5. URIを含むLSPデータ構造の変換

URI変換はメソッド単位の個別実装だけに依存せず、LSP 3.18に定義されたデータ構造を基準に管理する。

各メッセージのRequest parameters、Response result、Notification parametersに含まれるURIを抽出し、通信方向に応じて変換する。

変換対象は以下の分類で管理する。

##### A. Document URI関連

| データ構造 | フィールド | 変換方向 |
|---|---|---|
| `TextDocumentIdentifier` | `uri` | 双方向 |
| `VersionedTextDocumentIdentifier` | `uri` | 双方向 |
| `TextDocumentItem` | `uri` | 双方向 |
| `WorkspaceFolder` | `uri` | 双方向 |
| `OptionalVersionedTextDocumentIdentifier` | `uri` | 双方向 |
| `NotebookDocumentIdentifier` | `uri` | 双方向 |
| `VersionedNotebookDocumentIdentifier` | `uri` | 双方向 |
| `NotebookDocument` | `uri` | 双方向 |
| `NotebookCell` | `document` | 双方向 |

##### B. Location関連

| データ構造 | フィールド | 変換方向 |
|---|---|---|
| `Location` | `uri` | 双方向 |
| `LocationLink` | `targetUri` | Server → Client |
| `WorkspaceSymbol` | `location.uri` | Server → Client |
| `CallHierarchyItem` | `uri` | Server → Client |
| `TypeHierarchyItem` | `uri` | Server → Client |
| `DiagnosticRelatedInformation` | `location.uri` | Server → Client |

`Location`はリクエストパラメータとしてClientからServerへ渡されるケースもあるため、方向を固定せず、メッセージの実際の送信方向に従う。

##### C. WorkspaceEdit関連

| データ構造 | フィールド | 変換方向 |
|---|---|---|
| `WorkspaceEdit` | `changes`のURIキー | 双方向 |
| `TextDocumentEdit` | `textDocument.uri` | 双方向 |
| `CreateFile` | `uri` | 双方向 |
| `RenameFile` | `oldUri` / `newUri` | 双方向 |
| `DeleteFile` | `uri` | 双方向 |

`WorkspaceEdit`は専用の変換処理を実装する。

`changes`のMapキーも変換対象とする。

`TextEdit`の`range`と`newText`は変更しない。

##### D. DocumentLink関連

| データ構造 | フィールド | 変換方向 |
|---|---|---|
| `DocumentLink` | `target` | Server → Client |

`target`は一般URIであるため、File URIのみを変換する。

外部Web URLなどは変更しない。

##### E. Initialize / Workspace関連

| メッセージ | フィールド | 変換方向 |
|---|---|---|
| `initialize` | `params.rootUri` | Client → Server |
| `initialize` | `params.rootPath` | Client → Server |
| `initialize` | `params.workspaceFolders[].uri` | Client → Server |
| `workspace/didChangeWorkspaceFolders` | `event.added[].uri` | Client → Server |
| `workspace/didChangeWorkspaceFolders` | `event.removed[].uri` | Client → Server |
| `workspace/workspaceFolders` | `result[].uri` | Server → Client |

##### F. その他のURIを含むフィールド

| メッセージ・構造 | フィールド | 変換方向 |
|---|---|---|
| `textDocument/publishDiagnostics` | `params.uri` | Server → Client |
| `textDocument/didOpen` | `params.textDocument.uri` | Client → Server |
| `textDocument/didChange` | `params.textDocument.uri` | Client → Server |
| `textDocument/didSave` | `params.textDocument.uri` | Client → Server |
| `textDocument/didClose` | `params.textDocument.uri` | Client → Server |
| `textDocument/willSave` | `params.textDocument.uri` | Client → Server |
| `textDocument/willSaveWaitUntil` | `params.textDocument.uri` | Client → Server |
| `workspace/didChangeWatchedFiles` | `params.changes[].uri` | Client → Server |
| `window/showDocument` | `params.uri` | Server → Client |

##### G. Language FeaturesのRequest parameters

以下は代表的なURI-bearing fieldである。

| メソッド | フィールド | 変換方向 |
|---|---|---|
| `textDocument/completion` | `textDocument.uri` | Client → Server |
| `textDocument/hover` | `textDocument.uri` | Client → Server |
| `textDocument/signatureHelp` | `textDocument.uri` | Client → Server |
| `textDocument/declaration` | `textDocument.uri` | Client → Server |
| `textDocument/definition` | `textDocument.uri` | Client → Server |
| `textDocument/typeDefinition` | `textDocument.uri` | Client → Server |
| `textDocument/implementation` | `textDocument.uri` | Client → Server |
| `textDocument/references` | `textDocument.uri` | Client → Server |
| `textDocument/documentHighlight` | `textDocument.uri` | Client → Server |
| `textDocument/documentSymbol` | `textDocument.uri` | Client → Server |
| `textDocument/codeAction` | `textDocument.uri` | Client → Server |
| `textDocument/codeLens` | `textDocument.uri` | Client → Server |
| `textDocument/documentLink` | `textDocument.uri` | Client → Server |
| `textDocument/documentColor` | `textDocument.uri` | Client → Server |
| `textDocument/colorPresentation` | `textDocument.uri` | Client → Server |
| `textDocument/formatting` | `textDocument.uri` | Client → Server |
| `textDocument/rangeFormatting` | `textDocument.uri` | Client → Server |
| `textDocument/onTypeFormatting` | `textDocument.uri` | Client → Server |
| `textDocument/rename` | `textDocument.uri` | Client → Server |
| `textDocument/prepareRename` | `textDocument.uri` | Client → Server |
| `textDocument/foldingRange` | `textDocument.uri` | Client → Server |
| `textDocument/selectionRange` | `textDocument.uri` | Client → Server |
| `textDocument/linkedEditingRange` | `textDocument.uri` | Client → Server |
| `textDocument/semanticTokens/full` | `textDocument.uri` | Client → Server |
| `textDocument/semanticTokens/full/delta` | `textDocument.uri` | Client → Server |
| `textDocument/semanticTokens/range` | `textDocument.uri` | Client → Server |
| `textDocument/moniker` | `textDocument.uri` | Client → Server |
| `textDocument/prepareCallHierarchy` | `textDocument.uri` | Client → Server |
| `textDocument/prepareTypeHierarchy` | `textDocument.uri` | Client → Server |
| `textDocument/inlineValue` | `textDocument.uri` | Client → Server |
| `textDocument/inlayHint` | `textDocument.uri` | Client → Server |
| `textDocument/diagnostic` | `textDocument.uri` | Client → Server |
| `textDocument/inlineCompletion` | `textDocument.uri` | Client → Server |

上記は主要な例であり、完全な変換対象一覧はLSP 3.18の仕様定義を基準に管理する。

新しいメソッドやデータ構造を追加する場合は、URIを含むかどうかを確認し、必要な変換とテストを追加する。

#### 4.1.6. processIdの調整

LSPの`initialize`リクエストに含まれる`processId`を調整する。

`processId`はLSPクライアントのプロセスIDを表す。

ローカルのNeovimのPIDをリモート環境に渡しても、リモート側から参照できるとは限らない。

また、Proxy自身のPIDを渡すことも適切ではない。

そのため、初期実装では`processId`を`null`に変更する。

```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "processId": null
  }
}
```

Proxyはローカルクライアントの終了を検知し、リモートプロセスの終了管理を行う。

#### 4.1.7. リモートWorking Directoryの指定

リモートでLanguage Serverを起動する際に、対応するWorkspaceのパスをWorking Directoryとして指定する。

例：

```text
Local Workspace:
  /home/user/project

Remote Workspace:
  /workspace/project
```

ローカルWorkspaceを基準にRemote Workspaceを解決する。

Language Serverの実行時には、リモート側のパスを利用する。

---

### 4.2. JSON-RPC / LSPメッセージの取り扱い

#### 4.2.1. 双方向通信

JSON-RPCのRequestとNotificationはClientとServerの双方から送信される可能性がある。

そのため、以下の通信を区別して処理する。

- Client → Server Request
- Client → Server Notification
- Server → Client Request
- Server → Client Notification
- Server → Client Response
- Client → Server Response

Request IDは変更せず、Responseとの対応関係を維持する。

#### 4.2.2. ServerからClientへのRequest

ServerからClientへのRequestも通常のRequestとして扱い、必要なURI変換を実施する。

代表的な例：

- `workspace/applyEdit`
- `window/showDocument`
- `client/registerCapability`
- `client/unregisterCapability`
- `workspace/workspaceFolders`
- `workspace/configuration`

Requestの方向に応じて、paramsまたはresultの変換を判断する。

例えば、`workspace/applyEdit`ではServerからClientへ渡される`WorkspaceEdit`のURIをRemote URIからLocal URIへ変換する。

#### 4.2.3. Responseの処理

Responseには独自の`method`が存在しないため、Request IDと対応するメソッド情報を管理する。

URI変換が必要なResponseについては、元のRequestと対応するLSP仕様上のResponse型を基準に変換する。

ただし、Responseの変換方向は送信方向だけで決まるものではない。

基本的には、メッセージ内のURIがどちらの名前空間を表しているかに基づいて判断する。

#### 4.2.4. 未知のメッセージ

未知のメソッドやサーバー固有の拡張メッセージは、原則として変更せずに透過する。

未知のフィールドについて、文字列の内容からURIであると推測して変換してはならない。

---

### 4.3. 追加で検討する要件

#### 4.3.1. ファイル監視の仲介

LSPサーバーは`workspace/didChangeWatchedFiles`などを利用してファイル変更を検知する。

LSPクライアント側でファイル監視を行う場合、ローカルとリモートのファイルシステムが同じ実体を参照していることを踏まえ、監視イベントのURIを変換して通知する必要がある。

検討事項：

- NeovimのLSPクライアントが提供するファイル監視機能
- `client/registerCapability`による動的登録
- ローカル側のファイル監視イベントのURI変換
- リモート側で発生した変更の検知
- マウント方式によるファイル監視の差異

初期実装では独自のファイル監視機構を追加せず、Neovim側の既存機能を利用できるかを確認する。

#### 4.3.2. 複数マウントポイント

Workspaceが複数のディレクトリに分かれている場合に対応できるようにする。

例：

| Local | Remote |
|---|---|
| `/home/user/project` | `/workspace/project` |
| `/home/user/shared` | `/workspace/shared` |

単一のWorkspace Rootを前提とせず、複数のPath Mappingを管理できる設計にする。

ただし、初期実装では単一のWorkspace Mappingから開始してよい。

#### 4.3.3. 拡張可能なメッセージ変換機構

`initializationOptions`や`workspace/configuration`などには、サーバー固有の設定値が含まれることがある。

その中にパスが含まれる可能性はあるが、LSP共通仕様ではその意味を判断できない。

そのため、初期実装ではこれらのフィールドを透過的に扱う。

将来的に必要となった場合に備えて、サーバー固有の変換処理を追加できる仕組みを検討する。

#### 4.3.4. OSが異なる場合のパス変換

ローカルとリモートでOSが異なる場合にも対応できるようにする。

例：

```text
Local:
  C:\Users\user\project

Remote:
  /workspace/project
```

初期実装ではLinux同士を優先してもよいが、パス変換処理をOS固有の文字列操作に依存させない。

---

## 5. LSP 3.18のその他の取り扱い

### 5.1. Capability Negotiation

ClientとServer間のCapability Negotiationは原則として透過する。

対象：

- `InitializeParams.capabilities`
- `InitializeResult.capabilities`
- `client/registerCapability`
- `client/unregisterCapability`

Proxyは、ClientとServer間で交渉される機能を勝手に追加・削除しない。

ただし、URI変換に必要なメッセージが動的登録された場合でも正しく処理できるようにする。

### 5.2. LSPライフサイクル

以下のメッセージを正しく中継する。

| メッセージ | 取り扱い |
|---|---|
| `initialize` | `processId`、URI関連フィールドを必要に応じて変換 |
| `initialized` | 原則透過 |
| `shutdown` | 原則透過 |
| `exit` | 原則透過。終了処理を連動 |
| `$/cancelRequest` | 原則透過 |
| `$/progress` | 原則透過 |
| `$/setTrace` | 原則透過 |
| `$/logTrace` | 原則透過 |

`shutdown`と`exit`の順序を維持する。

ローカルクライアントが終了した場合は、リモートのLanguage Serverも適切に終了させる。

### 5.3. URI以外のデータ

以下は、ローカルとリモートで同じファイル内容を参照する前提であれば、基本的に変換不要とする。

| 項目 | 理由 |
|---|---|
| Position | ファイル内容が同じであれば位置情報は共通 |
| Range | ファイル内容が同じであれば範囲情報は共通 |
| TextEdit | URI以外は共通 |
| Completion | ファイル内容や位置情報に依存するが変換不要 |
| Diagnostics | URI以外の情報は基本的に共通 |
| SemanticTokens | ファイル内容と位置情報が共通 |
| ドキュメント内容 | 同じファイル実体を参照するため転送不要 |
| initializationOptions | サーバー固有のデータであり、意味を推測して変更しない |
| workspace/configuration | サーバー固有の設定であり、原則透過 |
| workspace/didChangeConfiguration | 同上 |
| executeCommandのarguments | サーバー固有のデータであり、原則透過 |

ただし、これらの構造の中にURIを含む標準フィールドがある場合は、URI変換の対象とする。

例えば、`WorkspaceEdit`の`TextEdit`自体は変換不要だが、`WorkspaceEdit`が参照するURIは変換が必要となる。

---

## 6. アーキテクチャ

Proxyは以下の責務に分割する。

### 6.1. Protocol Adapter

LSP/JSON-RPCメッセージの解析と変換を担当する。

責務：

- JSON-RPCフレーミングの解析
- JSON-RPCメッセージの解析
- Request / Response / Notificationの識別
- Request IDとメソッドの対応管理
- URIフィールドの抽出
- URIの双方向変換
- `processId`の調整
- 変換後のメッセージのシリアライズ

LSPのメッセージ構造を理解するが、すべてのメッセージを独自に実装する必要はない。

LSP 3.18の既知のschemaに基づいて変換し、それ以外は透過的に扱う。

URI変換対象の型定義は、実装とテストの共通基準として管理する。

### 6.2. Transport

ローカルとリモートの標準入出力を中継する。

責務：

- 双方向のデータ転送
- バッファリング
- EOFの伝播
- Backpressureへの対応
- 通信終了の管理
- 標準エラー出力の分離

Protocol Adapterとの間では、メッセージ単位でデータを受け渡す。

### 6.3. Remote Process Manager

リモート環境でLanguage Serverを起動・管理する。

責務：

- リモートコマンドの実行
- 引数の指定
- Working Directoryの指定
- 環境変数の指定
- 標準入出力の取得
- プロセス終了の検知
- プロセス終了時の後処理

リモート環境の起動・停止管理と、Language Serverのプロセス管理は分離する。

### 6.4. Path Mapper

ローカルとリモートのパス・URIの対応関係を管理する。

責務：

- Local Path → Remote Path
- Remote Path → Local Path
- Local URI → Remote URI
- Remote URI → Local URI
- 複数のPath Mappingの管理
- URI schemeの判定
- パスの正規化と境界判定

### 6.5. Remote Backend

リモート実行環境固有の処理を担当する。

初期実装ではDev Container CLIを利用する。

責務：

- Dev Containerの識別
- 実行環境の状態確認
- 必要に応じた環境起動
- `devcontainer exec`によるコマンド実行
- リモート側のWorkspace解決

将来的にはSSHなど、別の実行方式を追加できる構造とする。

Protocol AdapterやPath MapperはDev Container固有の実装に依存させない。

---

## 7. 実行環境の取り扱い

### 7.1. コマンド実行

Language Serverのコマンドと引数はリモート環境で実行する。

実際のCLI引数はDev Container CLIの仕様に合わせる。

シェル文字列を組み立てるのではなく、コマンドと引数を分離して渡す。

### 7.2. 環境変数

ローカル環境の環境変数を無条件にリモートへ転送しない。

基本方針：

- PATHはリモート環境のものを利用する。
- HOMEはリモート環境のものを利用する。
- 必要な追加環境変数のみ明示的に指定する。
- ローカル固有の環境変数を不用意に持ち込まない。

### 7.3. プロセスID

ローカルとリモートではPIDの名前空間が異なる可能性がある。

そのため、ローカル側のPIDをリモート側のプロセス管理に利用しない。

LSPの`initialize.params.processId`は`null`に変更する。

---

## 8. エラー処理・ライフサイクル

以下のケースを考慮する。

| ケース | 想定する処理 |
|---|---|
| リモート環境の起動失敗 | エラーをstderrに出力し終了 |
| Language Serverの起動失敗 | エラーをstderrに出力し終了 |
| リモートプロセスの異常終了 | ローカルProxyを終了 |
| ローカルクライアントの終了 | リモートプロセスを終了 |
| stdioの切断 | 双方向の通信を停止 |
| JSON-RPCの解析失敗 | エラーを記録し、通信を安全に終了 |
| URI変換失敗 | 不正な変換を行わず、エラーとして扱う |

Proxyは異常終了時にリモート側のプロセスが残らないようにする。

ただし、Dev Container自体はLanguage Serverの終了に連動して停止させない。

コンテナは他のLanguage Serverやターミナルから共有される可能性があるためである。

---

## 9. 実装上の重要な設計判断

### 9.1. JSON-RPCの透過性

ProxyはLSPの全メソッドを個別に実装する必要はない。

JSON-RPCのメッセージ構造を解析し、変換が必要なメッセージだけを加工する。

それ以外のメッセージは可能な限り透過的に中継する。

### 9.2. URI変換の網羅性

URI変換は、単純な文字列検索・置換で実装してはならない。

LSP 3.18に定義されたURI-bearing fieldを明示的に管理し、メッセージ構造に応じて変換する。

以下を実施する。

- 仕様上のURI-bearing typeを洗い出す。
- 各型が利用されるRequest / Response / Notificationを特定する。
- Client → ServerとServer → Clientの方向を整理する。
- 変換対象の一覧を実装とテストで共有する。
- LSP仕様の更新時に変換対象を見直す。

未知の拡張フィールドについては、URIであると推測して変換しない。

### 9.3. Responseの変換

Responseには`method`が含まれないため、Request IDと対応するメソッド情報を管理する。

URI変換が必要なResponseについては、元のRequestと対応するLSP仕様上のResponse型を基準に変換する。

ただし、変換方向は単にメッセージの送信方向で決めるのではなく、URIが表す名前空間に基づいて判断する。

### 9.4. ファイル内容の整合性

ローカルとリモートが同じファイル実体を参照する前提なので、ファイル内容やPositionなどの情報は基本的に変換不要とする。

ただし、LSPの`didChange`などによる未保存の変更は、通常どおりLSPクライアントからサーバーに通知される。

### 9.5. Remote Backendの分離

Dev Container CLIの実装詳細をProxyのコアに持ち込まない。

Remote Backendのインターフェースを定義し、実行環境ごとの処理を分離する。

これにより、将来的にSSHなどの実行方式を追加できるようにする。

---

## 10. テスト方針

### 10.1. Transport

- 1メッセージが複数回の読み取りに分割されるケース
- 複数メッセージが1回の読み取りで届くケース
- 大きなメッセージの送受信
- EOFの伝播
- Backpressure
- stderrとstdoutの分離

### 10.2. Protocol Adapter

#### JSON-RPC

- Requestの中継
- Responseの中継
- Notificationの中継
- ServerからClientへのRequest
- Request IDの維持
- RequestとResponseの対応
- `$/cancelRequest`の中継

#### URI変換

- `initialize.rootUri`の変換
- `initialize.workspaceFolders`の変換
- `TextDocumentIdentifier`の変換
- `Location`の変換
- `LocationLink`の変換
- `WorkspaceEdit`の変換
- `DocumentLink`の変換
- `DiagnosticRelatedInformation`の変換
- `CallHierarchyItem`の変換
- `TypeHierarchyItem`の変換
- Notebook関連URIの変換
- 非File URIが変更されないこと
- Workspace外のURIが誤変換されないこと
- 未知のメッセージが透過されること

#### ライフサイクル・Capability

- `initialize`の`processId`変換
- `initialized`の中継
- `shutdown`の中継
- `exit`の中継
- Capability Negotiationの透過
- 動的Capability登録・解除の中継

### 10.3. Path Mapper

- Local → Remoteの変換
- Remote → Localの変換
- Workspace外のパス
- 複数マッピング
- URIエンコードされたパス
- ディレクトリ境界の判定
- 不正なURI
- OSごとのパス表現

### 10.4. Remote Backend

- リモートコマンドの起動
- 引数の受け渡し
- Working Directoryの指定
- 環境変数の指定
- プロセス終了の検知
- 起動失敗時の処理
- クライアント終了時の後処理

### 10.5. 結合テスト

実際のDev Container環境でLanguage Serverを起動し、以下を確認する。

- NeovimからリモートのLanguage Serverに接続できる。
- 補完・Diagnostics・定義ジャンプなどの基本機能が動作する。
- ローカルとリモートで異なるWorkspaceパスを利用できる。
- WorkspaceEditを使った編集が正しいローカルファイルに反映される。
- Language Server終了時にリモートプロセスが残らない。
- コンテナ自体はLanguage Server終了後も維持される。

---

## 11. 実装優先順位

初期実装は以下の順序で進める。

1. **Transport**
   - stdioの双方向中継
   - JSON-RPCフレーミング処理

2. **Remote Process Manager**
   - Dev Container CLI経由のコマンド実行
   - Working Directory指定
   - プロセス終了管理

3. **Path Mapper**
   - Local / Remoteのパスマッピング
   - File URIの双方向変換

4. **Protocol Adapter**
   - `initialize`の変換
   - `processId`の調整
   - LSP 3.18のURI-bearing fieldの変換
   - Request / Response / Notificationの双方向処理

5. **結合テスト**
   - Neovimから実際のLanguage Serverを利用する。

6. **拡張**
   - 複数マウントポイント
   - ファイル監視の調整
   - サーバー固有の変換処理
   - 別のRemote Backend

---

## 12. 現時点での結論

今回のケースでは、以下を設計上の要件とする。

### 必須

- リモートプロセスの起動・終了管理
- stdioの双方向中継
- JSON-RPCのフレーミング処理
- File URIの双方向変換
- LSP 3.18のURI-bearing fieldの網羅的な変換
- `rootUri` / `workspaceFolders`の変換
- `processId`の調整
- リモートWorking Directoryの指定
- Client / Server双方からのRequest / Notification / Responseの中継
- LSPライフサイクルの維持
- Capability Negotiationの透過

### 追加で検討

- ファイル監視の仲介
- 複数マウントポイント
- `initializationOptions`などに対する拡張可能な変換機構
- OSが異なる場合のパス変換

### 基本的に対象外

- LSPのPosition / Range
- ドキュメントの内容
- CompletionやDiagnosticsの構造
- サーバー固有の設定内容
- ファイル同期
- ファイルシステムの代理アクセス

特に重要なのは、以下の点である。

**JSON-RPCのすべてのメッセージを解析する必要はないが、LSP 3.18の仕様に定義されたURI-bearing fieldについては、双方向で網羅的に変換できる仕組みが必要である。**

そのため、URI変換対象はLSPのデータ構造を基準に管理し、Request / Response / Notificationのそれぞれにおける利用箇所を明確にする。

また、Dev ContainerはRemote Backendの一実装として扱い、Proxyのコアロジックから分離する。

これを基本方針として実装を進める。
