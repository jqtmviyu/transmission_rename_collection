### transmission 合集重命名

#### 配置

参考 `config.json.example` 创建 `config.json` 文件, 和可执行文件放到同一目录

#### transmission 配置

1. 如果只有一季, 里面没有嵌套文件夹, 则把需要重命名的种子命名成 `番名` 格式

![](https://img.081024.xyz/202502060202925.png)

默认为第一季, 如果需要手动指定季数, 则种子名称改为成 `番名`+`空格`+`Season 2` 格式, 复用`seasonPathRegex`配置

1. 如果有多季, 且里面嵌套的文件夹只有一层, 每个文件夹对应一季, 则把种子名称成 `番名`, 对应的文件夹命名成 `Season 0` `Season 1` 

![](https://img.081024.xyz/202502060201963.png)

1. 添加 `collection` 标签

#### 运行

```bash
./rename-collection
```

成功后 `collection` 标签会替换成 `rename`

#### 注意事项

1. 需要安装支持标签的transmission主题, 例如: [TrguiNG](https://github.com/openscopeproject/TrguiNG)
1. 不支持多层文件夹