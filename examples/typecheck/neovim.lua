-- Minimal standalone Neovim configuration for the k6 LSP wrapper.
--
-- K6_BIN=/absolute/path/to/k6 nvim \
--   -u examples/typecheck/neovim.lua /absolute/path/to/test.js
--
-- K6_BIN must name the custom k6 binary when the script imports an extension.

if vim.fn.has("nvim-0.11") ~= 1 then
  error("the k6 LSP example requires Neovim 0.11 or newer")
end

local function absolute_path(path)
  return vim.fs.normalize(vim.fn.fnamemodify(path, ":p"))
end

local configured_binary = vim.env.K6_BIN
if configured_binary == "" then
  configured_binary = nil
elseif configured_binary ~= nil then
  configured_binary = absolute_path(configured_binary)
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

    local buffer_path = absolute_path(buffer_name)
    local entry = buffer_path
    local root = vim.fs.root(entry, { ".git" }) or vim.fs.dirname(entry)
    if vim.fs.relpath(root, buffer_path) == nil then
      return
    end

    local k6 = configured_binary or vim.fs.joinpath(root, "k6")
    if vim.fn.executable(k6) ~= 1 then
      vim.notify_once(
        "k6 LSP was not started because k6 is not executable at " .. k6 .. "; build it or set K6_BIN",
        vim.log.levels.WARN
      )
      return
    end

    vim.lsp.start({
      name = "k6",
      cmd = { k6, "lsp", "--server", server, entry },
      cmd_cwd = root,
      root_dir = root,
    }, { bufnr = event.buf })
  end,
})
