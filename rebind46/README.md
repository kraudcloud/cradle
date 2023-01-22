listens to tcp ports being opened on ipv4 and mirrors the port on ipv6

it will only do that if the bind was on v4 address 0.0.0.0,
specifically this means not for localhost, and not if the program
decided to enumerate local interface addresses and pick one of those



this needs

CONFIG_FTRACE
CONFIG_FUNCTION_TRACER
CONFIG_FPROBE
DEBUG_INFO_BTF
BPF_JIT
