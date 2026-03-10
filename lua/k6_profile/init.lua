local M = {}

M.ns = vim.api.nvim_create_namespace("k6_profile_overlay")
M.state = {
  by_file = {},
  totals = {
    cpu_ns = 0,
    alloc_space = 0,
    alloc_objects = 0,
  },
  run = {
    active = false,
    file_path = "",
    started_at = 0,
    job_id = 0,
  },
}

local metric_palette = {
  "#5f6368",
  "#7a7055",
  "#9a8749",
  "#bfa03f",
  "#e0ba35",
  "#ffd75f",
  "#ffbf40",
  "#ff9f1a",
  "#ff7a00",
}

local function setup_highlights()
  local ok = pcall(vim.api.nvim_set_hl, 0, "K6ProfileStatus", {
    fg = "#00d7ff",
    bold = true,
  })
  if not ok then
    return
  end

  pcall(vim.api.nvim_set_hl, 0, "K6ProfileStatusRunning", {
    fg = "#9eff6e",
    bold = true,
  })

  for i, color in ipairs(metric_palette) do
    pcall(vim.api.nvim_set_hl, 0, "K6ProfileMetric" .. i, {
      fg = color,
      bold = i >= (#metric_palette - 2),
    })
  end
end

setup_highlights()

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

local function pct_num(part, total)
  if not part or not total or total <= 0 then
    return 0
  end
  return (part * 100.0) / total
end

local function share(part, total)
  if not part or not total or total <= 0 then
    return 0
  end
  return part / total
end

local function metric_hl(cpu_ns, alloc_space, alloc_objects)
  local combined_share = (
    share(cpu_ns, M.state.totals.cpu_ns) +
    share(alloc_space, M.state.totals.alloc_space) +
    share(alloc_objects, M.state.totals.alloc_objects)
  ) / 3.0

  local idx = math.floor(combined_share * (#metric_palette - 1)) + 1
  if idx < 1 then
    idx = 1
  end
  if idx > #metric_palette then
    idx = #metric_palette
  end

  return "K6ProfileMetric" .. idx, combined_share * 100.0
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
    return ""
  end
  local original = p
  local abs = vim.fn.fnamemodify(p, ":p")
  if not abs or abs == "" then
    abs = p
  end
  local real = vim.loop.fs_realpath(abs)
  if real and real ~= "" then
    return real
  end
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
  return out
end

local function find_stats_for_file(abs_path)
  for _, c in ipairs(path_candidates(abs_path)) do
    if M.state.by_file[c] then
      return M.state.by_file[c]
    end
  end
  for k, v in pairs(M.state.by_file) do
    if abs_path:sub(-#k) == k then
      return v
    end
  end
  return nil
end

local function running_status_suffix(abs_path)
  if not M.state.run.active then
    return ""
  end
  local elapsed = os.time() - (M.state.run.started_at or os.time())
  if elapsed < 0 then
    elapsed = 0
  end
  if abs_path ~= "" and abs_path == M.state.run.file_path then
    return string.format(" | RUNNING current file (%ds)", elapsed)
  end
  return string.format(" | RUNNING (%ds)", elapsed)
end

local function apply_all_loaded_buffers()
  for _, b in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(b) then
      M.apply(b)
    end
  end
end

function M.clear(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  vim.api.nvim_buf_clear_namespace(bufnr, M.ns, 0, -1)
end

function M.apply(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  M.clear(bufnr)

  local abs = normalize_abs_path(vim.api.nvim_buf_get_name(bufnr))
  local stats = nil
  if abs ~= "" then
    stats = find_stats_for_file(abs)
  end

  local status_text = "k6 profile overlay: ON (no profile data for this file)"
  if stats then
    local count = 0
    local file_cpu = 0
    local file_mem = 0
    local file_obj = 0
    for _, line_stats in pairs(stats) do
      count = count + 1
      file_cpu = file_cpu + (line_stats.cpu_ns or 0)
      file_mem = file_mem + (line_stats.alloc_space or 0)
      file_obj = file_obj + (line_stats.alloc_objects or 0)
    end
    status_text = string.format(
      "k6 profile overlay: ON (%d profiled lines, total CPU=%s, total MEM=%s, file sum CPU=%s MEM=%s OBJ=%s, inclusive attribution)",
      count,
      hr_duration_ns(M.state.totals.cpu_ns or 0),
      hr_bytes(M.state.totals.alloc_space or 0),
      string.format("%.2f%%", pct_num(file_cpu, M.state.totals.cpu_ns)),
      string.format("%.2f%%", pct_num(file_mem, M.state.totals.alloc_space)),
      string.format("%.2f%%", pct_num(file_obj, M.state.totals.alloc_objects))
    )
  end
  local running_suffix = running_status_suffix(abs)
  local status_hl = "K6ProfileStatus"
  if running_suffix ~= "" then
    status_text = status_text .. running_suffix
    status_hl = "K6ProfileStatusRunning"
  end
  vim.api.nvim_buf_set_extmark(bufnr, M.ns, 0, 0, {
    virt_text = { { status_text, status_hl } },
    virt_text_pos = "eol",
  })

  if not stats then
    return
  end

  for lnum, v in pairs(stats) do
    local row = tonumber(lnum) - 1
    if row and row >= 0 then
      local cpu_ns = v.cpu_ns or 0
      local alloc_space = v.alloc_space or 0
      local alloc_objects = v.alloc_objects or 0
      local hl, combined_pct = metric_hl(cpu_ns, alloc_space, alloc_objects)
      local vt = string.format(
        " CPU %s (%s) | MEM %s (%s) | OBJ %d (%s) | COMBINED %.2f%%",
        hr_duration_ns(cpu_ns),
        pct(cpu_ns, M.state.totals.cpu_ns),
        hr_bytes(alloc_space),
        pct(alloc_space, M.state.totals.alloc_space),
        alloc_objects,
        pct(alloc_objects, M.state.totals.alloc_objects),
        combined_pct
      )
      vim.api.nvim_buf_set_extmark(bufnr, M.ns, row, 0, {
        virt_text = { { vt, hl } },
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
  if type(decoded.totals) == "table" then
    totals.cpu_ns = decoded.totals.cpu_ns or 0
    totals.alloc_space = decoded.totals.alloc_space or 0
    totals.alloc_objects = decoded.totals.alloc_objects or 0
  else
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
  end
  M.state.totals = totals
  M.apply(0)
  vim.notify("k6 profile loaded", vim.log.levels.INFO)
end


function M.run_current_file()
  local bufnr = vim.api.nvim_get_current_buf()
  local file_path = normalize_abs_path(vim.api.nvim_buf_get_name(bufnr))
  if file_path == "" then
    vim.notify("No file in current buffer", vim.log.levels.ERROR)
    return
  end
  if vim.fn.filereadable(file_path) == 0 then
    vim.notify("Current file is not readable: " .. file_path, vim.log.levels.ERROR)
    return
  end

  if vim.bo[bufnr].modified then
    local ok, err = pcall(vim.cmd, "write")
    if not ok then
      vim.notify("Failed to save file before run: " .. tostring(err), vim.log.levels.ERROR)
      return
    end
  end

  local base = vim.fn.fnamemodify(file_path, ":t:r"):gsub("[^%w_-]", "_")
  local stamp = tostring(os.time())
  local pprof_path = string.format("/tmp/k6-js-cpu-%s-%s.pprof", base, stamp)
  local trace_path = string.format("/tmp/k6-js-trace-%s-%s.trace", base, stamp)
  local log_path = string.format("/tmp/k6-js-run-%s-%s.log", base, stamp)

  local cmd = {
    "k6",
    "run",
    "--js-profiling-enabled",
    "--js-profiling-scope=combined",
    "--js-cpu-profile-output=" .. pprof_path,
    "--js-runtime-trace-output=" .. trace_path,
    file_path,
  }

  M.state.run = {
    active = true,
    file_path = file_path,
    started_at = os.time(),
    job_id = 0,
  }
  apply_all_loaded_buffers()

  vim.notify("Running k6 profile for " .. file_path, vim.log.levels.INFO)

  local stdout_chunks = {}
  local stderr_chunks = {}
  local job_id = vim.fn.jobstart(cmd, {
    cwd = repo_root(),
    stdout_buffered = true,
    stderr_buffered = true,
    on_stdout = function(_, data)
      if type(data) == "table" then
        for _, line in ipairs(data) do
          if line and line ~= "" then
            table.insert(stdout_chunks, line)
          end
        end
      end
    end,
    on_stderr = function(_, data)
      if type(data) == "table" then
        for _, line in ipairs(data) do
          if line and line ~= "" then
            table.insert(stderr_chunks, line)
          end
        end
      end
    end,
    on_exit = function(_, code)
      local all = {}
      for _, line in ipairs(stdout_chunks) do
        table.insert(all, line)
      end
      for _, line in ipairs(stderr_chunks) do
        table.insert(all, line)
      end
      vim.fn.writefile(all, log_path)

      vim.schedule(function()
        M.state.run.active = false
        M.state.run.job_id = 0
        apply_all_loaded_buffers()

        if code ~= 0 then
          vim.notify(
            string.format("k6 run failed (exit %d). See %s", code, log_path),
            vim.log.levels.ERROR
          )
          return
        end
        if vim.fn.filereadable(pprof_path) == 0 then
          vim.notify(
            "k6 run finished but CPU profile was not found: " .. pprof_path,
            vim.log.levels.ERROR
          )
          return
        end
        M.load(pprof_path)
        vim.notify(
          string.format("k6 profile loaded from %s (trace: %s)", pprof_path, trace_path),
          vim.log.levels.INFO
        )
      end)
    end,
  })

  if job_id <= 0 then
    M.state.run.active = false
    M.state.run.job_id = 0
    apply_all_loaded_buffers()
    vim.notify("Failed to start k6 run", vim.log.levels.ERROR)
    return
  end

  M.state.run.job_id = job_id
  apply_all_loaded_buffers()
end

return M
