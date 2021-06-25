# webhook

> :notes: Sản phẩm được xây dựng bằng Go

webhook là công cụ hỗ trợ pentest. Các chức năng chỉ mang tính chất kiểm tra và tìm lỗi không dùng để tấn công. Các tính năng hiện có

* [x] Hứng reuqest HTTP, lưu trữ và hiển thị full request
* [x] XSS RCE
* [x] Fake Email
* [ ] Điều kiển máy tình từ xa.

## Cài đặt
### Yêu cầu
cài đặt golang, mongodb

### Sử dụng
Chỉnh sửa nội dung file `config.toml`
```
Listen = "127.0.0.1:5000"

User   = "admin"
Pass   = "admin"

MongoDatabase       = "reqhook"
MongoServer         = ["192.168.56.101:27017"]
MongoReplicaSetName = ""
MongoUser           = ""
MongoPassword       = ""
MongoAuthDB         = ""
```

chạy lệnh để thực thi
```bash
go run main.go
```

## Tính năng
### Request Hook
Mục đích: để kiểm tra các lỗi như xss, ssrf, rce, ... bằng cách nhận request http từ phía máy victim để check.
####
Trên victim tạo request truy cập link sau
```
http://[ip]/hook
```

Trên server truy cập link sau để xem các request
```
http://[ip]/admin/hook/view
```
![demo](https://github.com/namcuongq/webhook/blob/main/example/request.png)
### XSS RCE
Mục đích: RCE trên trình duyệt của victim để đỡ phải gửi đi gửi lại nhiều request. Kết nối 1 lần dùng mãi mãi.
Chỉnh sửa địa chỉ server trong file [xss_rce.js](https://github.com/namcuongq/webhook/blob/main/static/js/xss_rce.js). Tạo payload xss load script từ link `http://<ip>/static/js/xss_rce.js`
Trên server truy cập link sau để bắt đầu:
```
http://<ip>/admin/xss/channels
```
![demo](https://github.com/namcuongq/webhook/blob/main/example/xss.png)

### EMail Fake
Mục đích: Kiểm tra gửi email giả mạo với tên miền của victim
Truy cập link sau để gửi
```
http://<ip>/admin/email
```
