const std = @import("std");
const CrossTarget = std.zig.CrossTarget;

pub fn build(b: *std.build.Builder) void {
    const target = .{.cpu_arch = .wasm32, .os_tag = .wasi};
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "wasi",
        .root_source_file = .{ .path = "wasi.zig" },
        .target = target,
        .optimize = optimize,
    });

    exe.install();
}
