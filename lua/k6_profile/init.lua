local M = {}

M.ns = vim.api.nvim_create_namespace("k6_profile_overlay")
M.state = {
  by_file = {},
  totals = {
    cpu_ns = 0,
    alloc_space = 0,
    alloc_objects = 0,
  },
}

local function setup_highlights()
  local ok = pcall(vim.api.nvim_set_hl, 0, "K6ProfileStatus", {
    fg = "#00d7ff",
    bold = true,
  })
  if not ok then
    return
  end
  pcall(vim.api.nvim_set_hl, 0, "K6ProfileMetric", {
    fg = "#ffd75f",
    bold = true,
  })
end

setup_highlights()

local function debug_enabled()
  if vim.g.k6_profile_debug == nil then
    return true
  end
  return vim.g.k6_profile_debug == true or vim.g.k6_profile_debug == 1
end

local function debug_log(msg)
  if not debug_enabled() then
    return
  end
  vim.notify("[k6_profile] " .. msg, vim.log.levels.INFO)
end

local function hr_duration_ns(v)
  if v >= 1000000000 then
    return string.format("%.2fs", v / 1000000000)
  end
  if v >= 1000000 then
    return string.format("%.2fms", v / 1000000)
  end
  if v >= 1000 then
    return string.format("%.2fus", v / 1000)
  end
  return tostring(v) .. "ns"
end

local function hr_bytes(v)
  if v >= 1024 * 1024 * 1024 then
    return string.format("%.2fGB", v / (1024 * 1024 * 1024))
  end
  if v >= 1024 * 1024 then
    return string.format("%.2fMB", v / (1024 * 1024))
  end
  if v >= 1024 then
    return string.format("%.2fKB", v / 1024)
  end
  return tostring(v) .. "B"
end

local function pct(part, total)
  if not part or not total or total <= 0 then
    return "0.00%"
  end
  return string.format("%.2f%%", (part * 100.0) / total)
end

local function repo_root()
  -- local start = vim.fn.expand("%:p:h")
  -- if start == nil or start == "" then
  --   start = vim.loop.cwd()
  -- end
  -- local git = vim.fs.find(".git", { upward = true, path = start })[1]
  -- if not git then
    return vim.loop.cwd()
  -- end
  -- return vim.fs.dirname(git)
end

local function normalize_abs_path(p)
  if not p or p == "" then
    debug_log("normalize_abs_path: empty input")
    return ""
  end
  local original = p
  local abs = vim.fn.fnamemodify(p, ":p")
  if not abs or abs == "" then
    abs = p
  end
  local real = vim.loop.fs_realpath(abs)
  if real and real ~= "" then
    debug_log(string.format("normalize_abs_path: '%s' -> abs='%s' -> real='%s'", original, abs, real))
    return real
  end
  debug_log(string.format("normalize_abs_path: '%s' -> abs='%s' (no realpath)", original, abs))
  return abs
end

local function path_candidates(path)
  path = normalize_abs_path(path)
  local out = {}
  local seen = {}
  local function add(p)
    if p and p ~= "" and not seen[p] then
      seen[p] = true
      table.insert(out, p)
    end
  end

  add(path)
  add(vim.fn.fnamemodify(path, ":t"))
  local root = repo_root()
  local rel = vim.fn.fnamemodify(path, ":.")
  if rel and rel ~= path then
    add(rel)
    add("/" .. rel)
  end
  if root and root ~= "" then
    local from_root = path:gsub("^" .. vim.pesc(root) .. "/", "")
    add(from_root)
    add("/" .. from_root)
  end
  debug_log("path_candidates: " .. table.concat(out, " | "))
  return out
end

local function find_stats_for_file(abs_path)
  debug_log("find_stats_for_file: input=" .. tostring(abs_path))
  for _, c in ipairs(path_candidates(abs_path)) do
    if M.state.by_file[c] then
      debug_log("find_stats_for_file: direct match=" .. c)
      return M.state.by_file[c]
    end
  end
  for k, v in pairs(M.state.by_file) do
    if abs_path:sub(-#k) == k then
      debug_log("find_stats_for_file: suffix match=" .. k)
      return v
    end
  end
  debug_log("find_stats_for_file: no match")
  return nil
end

function M.clear(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  vim.api.nvim_buf_clear_namespace(bufnr, M.ns, 0, -1)
end

function M.apply(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  M.clear(bufnr)

  local abs = normalize_abs_path(vim.api.nvim_buf_get_name(bufnr))
  debug_log("apply: buffer normalized path=" .. tostring(abs))
  local stats = nil
  if abs ~= "" then
    stats = find_stats_for_file(abs)
  end

  local status_text = "k6 profile overlay: ON (no profile data for this file)"
  if stats then
    local count = 0
    for _ in pairs(stats) do
      count = count + 1
    end
    status_text = string.format(
      "k6 profile overlay: ON (%d profiled lines, total CPU=%s, total MEM=%s)",
      count,
      hr_duration_ns(M.state.totals.cpu_ns or 0),
      hr_bytes(M.state.totals.alloc_space or 0)
    )
  end
  vim.api.nvim_buf_set_extmark(bufnr, M.ns, 0, 0, {
    virt_text = { { status_text, "K6ProfileStatus" } },
    virt_text_pos = "eol",
  })

  if not stats then
    return
  end

  for lnum, v in pairs(stats) do
    local row = tonumber(lnum) - 1
    if row and row >= 0 then
      local vt = string.format(
        " CPU %s (%s) | MEM %s (%s) | OBJ %d (%s)",
        hr_duration_ns(v.cpu_ns or 0),
        pct(v.cpu_ns or 0, M.state.totals.cpu_ns),
        hr_bytes(v.alloc_space or 0),
        pct(v.alloc_space or 0, M.state.totals.alloc_space),
        v.alloc_objects or 0,
        pct(v.alloc_objects or 0, M.state.totals.alloc_objects)
      )
      vim.api.nvim_buf_set_extmark(bufnr, M.ns, row, 0, {
        virt_text = { { vt, "K6ProfileMetric" } },
        virt_text_pos = "eol",
      })
    end
  end
end

function M.load(pprof_path)
  if not pprof_path or pprof_path == "" then
    vim.notify("Usage: K6ProfileLoad <path-to-pprof>", vim.log.levels.ERROR)
    return
  end
  local root = repo_root()
  local helper = root .. "/cmds/k6_profile_export/main.go"
  if vim.fn.filereadable(helper) == 0 then
    vim.notify("Helper not found: " .. helper, vim.log.levels.ERROR)
    return
  end

  local cmd = {
    "go",
    "run",
    helper,
    "-pprof",
    pprof_path,
  }
  local out = vim.fn.system(cmd)
  if vim.v.shell_error ~= 0 then
    vim.notify("Failed to load profile: " .. out, vim.log.levels.ERROR)
    return
  end

  local ok, decoded = pcall(vim.json.decode, out)
  if not ok or type(decoded) ~= "table" or type(decoded.files) ~= "table" then
    vim.notify("Invalid JSON returned by helper", vim.log.levels.ERROR)
    return
  end

  M.state.by_file = decoded.files
  local totals = {
    cpu_ns = 0,
    alloc_space = 0,
    alloc_objects = 0,
  }
  for _, file_stats in pairs(M.state.by_file) do
    if type(file_stats) == "table" then
      for _, line_stats in pairs(file_stats) do
        if type(line_stats) == "table" then
          totals.cpu_ns = totals.cpu_ns + (line_stats.cpu_ns or 0)
          totals.alloc_space = totals.alloc_space + (line_stats.alloc_space or 0)
          totals.alloc_objects = totals.alloc_objects + (line_stats.alloc_objects or 0)
        end
      end
    end
  end
  M.state.totals = totals
  debug_log(string.format(
    "load: totals cpu=%d mem=%d obj=%d",
    totals.cpu_ns,
    totals.alloc_space,
    totals.alloc_objects
  ))
  M.apply(0)
  vim.notify("k6 profile loaded", vim.log.levels.INFO)
end

return M
