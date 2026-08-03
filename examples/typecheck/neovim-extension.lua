-- Standalone Neovim configuration for extension.js and the custom example binary.

local source = debug.getinfo(1, "S").source
if not vim.startswith(source, "@") then
  error("could not determine the Neovim configuration path")
end

local config_path = vim.fs.normalize(vim.fn.fnamemodify(source:sub(2), ":p"))
local examples_dir = vim.fs.dirname(config_path)
local root = vim.fs.normalize(vim.fs.joinpath(examples_dir, "..", ".."))

if vim.env.K6_BIN == nil or vim.env.K6_BIN == "" then
  vim.env.K6_BIN = vim.fs.joinpath(root, "k6-with-types")
end
if vim.env.K6_LSP_ENTRY == nil or vim.env.K6_LSP_ENTRY == "" then
  vim.env.K6_LSP_ENTRY = vim.fs.joinpath(examples_dir, "extension.js")
end

dofile(vim.fs.joinpath(examples_dir, "neovim.lua"))
