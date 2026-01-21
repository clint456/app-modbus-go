## 主要改进：

1. **文件输出支持**：
   - 添加 `fileHandle` 和 `filePath` 字段来管理文件句柄
   - 使用 `io.MultiWriter` 同时写入控制台和文件

2. **新的配置结构**：
   - `LoggerConfig` 结构体用于灵活配置
   - 支持配置日志级别、文件路径、是否启用控制台输出

3. **新的构造函数**：
   - `NewClient()` - 兼容原有接口（仅输出到控制台）
   - `NewClientWithFile()` - 简化版，同时输出到控制台和文件
   - `NewClientWithConfig()` - 完整配置版本

4. **自动创建目录**：
   - 如果日志文件路径的目录不存在，会自动创建

5. **资源管理**：
   - 添加 `Close()` 方法用于关闭文件句柄
   - 在接口中也添加了 `Close()` 方法

## 使用示例：

```go
// 方式1: 仅输出到控制台（兼容原有代码）
logger := logger.NewClient("INFO")

// 方式2: 同时输出到控制台和文件
logger, err := logger.NewClientWithFile("DEBUG", "/var/log/myapp/app.log")
if err != nil {
    panic(err)
}
defer logger.Close()

// 方式3: 使用完整配置（仅输出到文件）
logger := logger.NewClientWithConfig(logger.LoggerConfig{
    LogLevel:      "INFO",
    FilePath:      "/var/log/myapp/app.log",
    EnableConsole: false, // 不输出到控制台
})
defer logger.Close()

// 使用logger
logger.Info("Application started")
logger.Debugf("Processing %d items", 10)
```

这个改进保持了向后兼容性，同时增加了文件输出功能。记得在程序退出时调用 `Close()` 方法来确保文件正确关闭。