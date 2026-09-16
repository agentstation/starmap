package valkey

// Inspect role, expiry, shape, and size before reading or changing record bytes.
const inspectRecordScript = `
local replication = redis.call('INFO', 'replication')
if not string.find(replication, '\r\nrole:master\r\n', 1, true) then
    return {'replica'}
end
local ttl = redis.call('PTTL', KEYS[1])
if ttl >= 0 then return {'expiring'} end
local exists = ttl ~= -2
local version = ''
if exists then
    if redis.call('TYPE', KEYS[1]).ok ~= 'hash' then return {'invalid'} end
    if redis.call('HLEN', KEYS[1]) ~= 3 then return {'invalid'} end
    if redis.call('HGET', KEYS[1], 'schema') ~= '1' then return {'invalid'} end
    version = redis.call('HGET', KEYS[1], 'version')
    if not version or #version == 0 or #version > 128 then return {'invalid'} end
    if redis.call('HEXISTS', KEYS[1], 'data') ~= 1 then return {'invalid'} end
    if redis.call('HSTRLEN', KEYS[1], 'data') > tonumber(ARGV[1]) then return {'oversized'} end
end
`

const readRecordScript = inspectRecordScript + `
if not exists then return {'missing'} end
return {'ok', version, redis.call('HGET', KEYS[1], 'data')}
`

const writeRecordScript = inspectRecordScript + `
if ARGV[2] == '' then
    if exists then return {'conflict', version} end
elseif not exists or version ~= ARGV[2] then
    return {'conflict', version}
end
redis.call('HSET', KEYS[1], 'schema', '1', 'version', ARGV[3], 'data', ARGV[4])
return {'ok', ARGV[3], ''}
`
