[![Build Status](https://travis-ci.org/dearcode/sapper.svg?branch=master)](https://travis-ci.org/dearcode/sapper)  

# sapper
一个完整的Faas框架，用于提高开发效率、统一对外接口形式，完善监控、日志等等重复且重要的功能  

## 架构图  
![Sapper](/docs/sapper.png?raw=true "sapper")  

## 使用手册    
1.在manager页面中添加项目信息，包含项目源码所在git地址  
2.在git服务器中添加webhook  
3.当用户提交代码到git中时通过webhook触发distributor编译发布程序到指定服务器并启动运行  
4.应用启动后自动注册到etcd中  
5.当distributor监听到应用上线后注册相应接口到manager中  
6.当客户端请求到来时repeater读取db中接口信息验证权限及etcd中状态选择后端节点，进行转发  
