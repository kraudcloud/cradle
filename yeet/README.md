yeet
====

message passing for distributed system

- aggressive timeouts for liveness detection
- zero allocations


### protocol

| len |                                |
|-----|--------------------------------|
| 4   | little endian key              |
| 1   | ignored for future use         |
| 1   | user flags                     |
| 2   | little endian value size       |
| ..  | value                          |


### reserved keys

| key   |                                  |
|-------|----------------------------------|
| 0     | invalid                          |
| 1     | hello                            |
| 2     | ping                             |
| 3     | pong                             |
| 4     | close                            |
| 22    | sync                             |
| ..10  | invalid for future use           |
| ..255 | ignored for future use           |



### sync

write flooded when not synced
[ 22 0 0 0 21 0 0 0 ]
when synced
[ 22 0 0 0 06 0 0 0 ]
if both receive synced, sync is done and we send hello (1)

trailing message type 22 is ignored after sync is done
