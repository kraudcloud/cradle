periodic state exchange in go

Very simple eventual-delivery of absolute state over tcp.

a simple client/server that handles reconnects under the hood but *not* redelivery,
since the intent is to be used for absolute state, where redelivering
outdated messages would be pointless.

 - aggressive timeouts
 - automatically reconnects on failure
 - Read() will always succeed eventually, but might have missed some messages in the middle
 - Write() will usually succeed eventually, but doesn't guarantee the receiver actually read the message
