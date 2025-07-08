local socket = require("socket")
local time = socket.gettime()*1000
math.randomseed(time)
math.random(); math.random(); math.random()

local charset = {'q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p', 'a', 's',
  'd', 'f', 'g', 'h', 'j', 'k', 'l', 'z', 'x', 'c', 'v', 'b', 'n', 'm', 'Q',
  'W', 'E', 'R', 'T', 'Y', 'U', 'I', 'O', 'P', 'A', 'S', 'D', 'F', 'G', 'H',
  'J', 'K', 'L', 'Z', 'X', 'C', 'V', 'B', 'N', 'M', '1', '2', '3', '4', '5',
  '6', '7', '8', '9', '0'}

local decset = {'1', '2', '3', '4', '5', '6', '7', '8', '9', '0'}

-- load env vars
local max_user_index = tonumber(os.getenv("max_user_index")) or 962

local function stringRandom(length)
  if length > 0 then
    return stringRandom(length - 1) .. charset[math.random(1, #charset)]
  else
    return ""
  end
end

local function decRandom(length)
  if length > 0 then
    return decRandom(length - 1) .. decset[math.random(1, #decset)]
  else
    return ""
  end
end


local function compose_post()
  local boundary = "----LuaFormBoundary" .. stringRandom(16)
  local boundary_line = "--" .. boundary
  local body_parts = {}

  local function add_field(name, value)
    table.insert(body_parts, "--" .. boundary)
    table.insert(body_parts, 'Content-Disposition: form-data; name="' .. name .. '\r\n')
    table.insert(body_parts, value)
  end

  local function add_file(name, filename, content_type, data)
    table.insert(body_parts, "--" .. boundary)
    table.insert(body_parts,'Content-Disposition: form-data; name="' .. name .. '"; filename="' .. filename .. '"')
    table.insert(body_parts, 'Content-Type: ' .. content_type .. '\r\n')
    table.insert(body_parts, data)
  end

  local user_index = math.random(0, max_user_index - 1)
  local username = "username_" .. tostring(user_index)
  local user_id = tostring(user_index)
  local text = stringRandom(256)
  local num_user_mentions = math.random(0, 5)
  local num_urls = math.random(0, 5)
  local num_media = math.random(0, 4)


  for i = 0, num_user_mentions, 1 do
    local user_mention_id
    while (true) do
      user_mention_id = math.random(0, max_user_index - 1)
      if user_index ~= user_mention_id then
        break
      end
    end
    text = text .. " @username_" .. tostring(user_mention_id)
  end

  for i = 0, num_urls, 1 do
    text = text .. " http://" .. stringRandom(64)
  end

  -- Add form fields
  add_field("user_id", tostring(user_id))
  add_field("text", text)

  for i = 0, num_media - 1 do
    local bytes = {}
    for j = 1, 1024 do
      bytes[#bytes + 1] = string.char(math.random(0, 255))
    end
    local img_data = table.concat(bytes)
    add_file("images", "image_" .. i .. ".jpg", "image/jpeg", img_data)
  end

    -- Finish boundary
  table.insert(body_parts, "--" .. boundary .. "--\r\n")

  -- Combine body as a single string
  local body = table.concat(body_parts, "\r\n")

  local method = "POST"
  local path = "http://localhost:8080/save"
  local headers = {}
  headers["Content-Type"] = "application/x-www-form-urlencoded"

  return wrk.format(method, path, headers, body)
end

local function read_user_timeline()
  local user_id = tostring(math.random(0, max_user_index - 1))
  local start = tostring(time)
  local stop = tostring(start + 10)
  local user_ti = tostring(true) 
  local args = "user_id=" .. user_id .. "user_ti=" .. user_ti 

  local method = "GET"
  
  local headers = {}
  headers["Content-Type"] = "application/x-www-form-urlencoded"
  local path = "http://localhost:8080/timeline" .. args
  return wrk.format(method, path, headers, nil)
end

local function read_home_timeline()
    local user_id = tostring(math.random(0, max_user_index - 1))
    local start = tostring(math.random(0, 100))
    local stop = tostring(start + 10)
    local user_ti = tostring(false)
    local args = "user_id=" .. user_id .. "user_ti=" .. user_ti 
    local method = "GET"
    local headers = {}
    headers["Content-Type"] = "application/x-www-form-urlencoded"
    local path = "http://localhost:8080/timeline" .. args
    return wrk.format(method, path, headers, nil)
  end

request = function()
    cur_time = math.floor(socket.gettime())
    local read_home_timeline_ratio = 0.60
    local read_user_timeline_ratio = 0.30
    local compose_post_ratio       = 0.10

    local coin = math.random()
    if coin < read_home_timeline_ratio then
      return read_home_timeline()
    elseif coin < read_home_timeline_ratio + read_user_timeline_ratio then
      return read_user_timeline()
    else
      return compose_post()
    end
  end
