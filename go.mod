module github.com/ishizakahiroshi/doorpost

go 1.22

// 設定を TOML で読むための唯一の外部依存。
// 設定にコメントが書けないと、鍵と許可 IP の横に「それが何の鍵か」を残せない。
require github.com/BurntSushi/toml v1.4.0
