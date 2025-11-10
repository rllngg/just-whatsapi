This will document about event


follow @event.go


# Publisher
[CHECKED] Event will be published when device created (manager.go:124)
[CHECKED] Event will be published when device is disconnected (internet issue) (manager.go:79-84)
[CHECKED] Event will be published when device is logout (manager.go:86-95)
[CHECKED] Event will be published new message (from whatsapp) with atleast payload of device_id, message info, source (manager.go:67-77)
[CHECKED] Event will be published new message (from http handler) with atleast payload of device_id, message info, source (manager.go:298-302, 365-369, 432-436)
