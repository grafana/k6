-- Minimal standalone Neovim configuration for the k6 LSP wrapper.
--
-- K6_BIN=/absolute/path/to/k6 nvim \
--   -u examples/typecheck/neovim.lua /absolute/path/to/test.js
--
-- K6_BIN must name the custom k6 binary when the project imports an extension.
-- Set K6_LSP_ROOT only when the workspace differs from Neovim's startup directory.

if vim.fn.has("nvim-0.11") ~= 1 then
  error("the k6 LSP example requires Neovim 0.11 or newer")
end

local function absolute_path(path)
  return vim.fs.normalize(vim.fn.fnamemodify(path, ":p"))
end

local launch_directory = absolute_path(vim.fn.getcwd())

local configured_binary = vim.env.K6_BIN
if configured_binary == "" then
  configured_binary = nil
elseif configured_binary ~= nil then
  configured_binary = absolute_path(configured_binary)
end

local path_binary = vim.fn.exepath("k6")
if path_binary == "" then
  path_binary = nil
else
  path_binary = absolute_path(path_binary)
end

local configured_workspace = vim.env.K6_LSP_ROOT
if configured_workspace == nil or configured_workspace == "" then
  configured_workspace = launch_directory
elseif configured_workspace ~= nil then
  configured_workspace = absolute_path(configured_workspace)
end

local server = vim.env.K6_LSP_SERVER
if server == nil or server == "" then
  server = "tsgo"
end

local group = vim.api.nvim_create_augroup("k6-lsp-example", { clear = true })
vim.api.nvim_create_autocmd("LspAttach", {
  group = group,
  callback = function(event)
    local client = vim.lsp.get_client_by_id(event.data.client_id)
    if client == nil or client.name ~= "k6" then
      return
    end

    local function map(mode, lhs, rhs, description)
      vim.keymap.set(mode, lhs, rhs, { buffer = event.buf, desc = description })
    end

    map("n", "K", vim.lsp.buf.hover, "k6 LSP hover")
    map("n", "gd", vim.lsp.buf.definition, "k6 LSP definition")
    map("n", "grr", vim.lsp.buf.references, "k6 LSP references")
    map("n", "grn", vim.lsp.buf.rename, "k6 LSP rename")

    if client:supports_method("textDocument/completion") then
      vim.lsp.completion.enable(true, client.id, event.buf, { autotrigger = true })
    end
  end,
})

vim.api.nvim_create_autocmd("FileType", {
  group = group,
  pattern = { "javascript", "javascriptreact", "typescript", "typescriptreact" },
  callback = function(event)
    local buffer_name = vim.api.nvim_buf_get_name(event.buf)
    if buffer_name == "" then
      return
    end

    local workspace = configured_workspace
    if vim.fn.isdirectory(workspace) ~= 1 then
      vim.notify_once(
        "k6 LSP was not started because K6_LSP_ROOT is not a directory: " .. workspace,
        vim.log.levels.WARN
      )
      return
    end

    local k6 = configured_binary or path_binary or vim.fs.joinpath(workspace, "k6")
    if vim.fn.executable(k6) ~= 1 then
      vim.notify_once(
        "k6 LSP was not started because k6 is not executable at "
          .. k6
          .. "; build it, add k6 to PATH, or set K6_BIN",
        vim.log.levels.WARN
      )
      return
    end

    vim.lsp.start({
      name = "k6",
      cmd = { k6, "lsp", "--server", server, workspace },
      cmd_cwd = workspace,
      root_dir = workspace,
    }, { bufnr = event.buf })
  end,
})
