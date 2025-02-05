### transmission 合集重命名

#### 配置

参考 `config.json.example` 创建 `config.json` 文件, 和可执行文件放到同一目录

#### transmission 配置

1. 把需要重命名的种子命名成 `番名 - 季度` 格式

例如:  `[DMG][Yofukashi_no_Uta][01-13 END][1080P][GB][MP4]` 改为 `彻夜之歌 - S01`

2. 添加 `collection` 标签

#### 运行

```bash
./rename-collection
```

成功后 `collection` 标签会替换成 `rename`

#### 注意事项

1. 需要安装支持标签的transmission主题, 例如: [TrguiNG](https://github.com/openscopeproject/TrguiNG)
1. 暂时只支持单层文件夹,不支持多文件夹

![](https://img.081024.xyz/20250205220047.png)