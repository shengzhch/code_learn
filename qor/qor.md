# QOR

### GOLANG 抽象了业务应用程序、内容管理系统(CMS)和电子商务系统(EC)所需的公共特性。

##  QOR Admin 内容管理系统(CMS)

### QOR Authentication (身份验证)

* QOR提供了一个认证系统Auth，它是一个用于Golang web开发的模块化认证系统，它提供了不同的认证后端来加速您的开发

* 目前Auth有数据库密码、github、谷歌、facebook、twitter身份验证支持，并且很容易添加其他基于Auth接口

* Auth使用Render来呈现页面，您可以参考它来了解如何注册func映射、扩展视图路径，如果您想将应用程序编译成二进制文件，请务必参考BindataFS。

* 如果您有视图路径，您可以将它们添加到viewpath中，如果您想要覆盖默认(丑陋的)登录/注册页面，或者开发诸如 https://github.com/qor/auth_themes 这样的auth主题

* 在一些Auth操作之后，比如登录、注册或确认，Auth将把用户重定向到某个URL，您可以配置使用Redirector重定向哪个页面，默认情况下，redirct将重定向到主页。如果你想重定向到最后访问的页面，redirect_back是为你，你可以配置它，并使用它作为重定向器。

* 顺便说一下，为了使它正确工作，redirect_back需要为每个请求保存最后一个被访问的URL到会话管理器的会话中，这意味着，您需要将redirect_back和SessionManager的中间件挂载到路由器中。

### QOR Authorization （权限管理)

### Resources
* 资源是可以通过QOR管理员的用户界面(通常是GORM后端模型)进行管理。





