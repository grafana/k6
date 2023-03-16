use std::env;
use std::fs;
use std::io;
use std::io::Write;
use std::process::exit;

// Until NotADirectory is implemented, read the underlying error raised by
// wasi-libc. See https://github.com/rust-lang/rust/issues/86442
use libc::ENOTDIR;

fn main() {
    let args: Vec<String> = env::args().collect();

    match args[1].as_str() {
        "ls" => {
            main_ls(&args[2]);
            if args.len() == 4 && args[3].as_str() == "repeat" {
                main_ls(&args[2]);
            }
        }
        "stat" => {
            main_stat();
        }
        _ => {
            writeln!(io::stderr(), "unknown command: {}", args[1]).unwrap();
            exit(1);
        }
    }
}

fn main_ls(dir_name: &String) {
    match fs::read_dir(dir_name) {
        Ok(paths) => {
            for ent in paths.into_iter() {
                println!("{}", ent.unwrap().path().display());
            }
        }
        Err(e) => {
            if let Some(error_code) = e.raw_os_error() {
                if error_code == ENOTDIR {
                    println!("ENOTDIR");
                } else {
                    println!("errno=={}", error_code);
                }
            } else {
                println!("unknown error");
            }
        }
    }
}

extern crate libc;

fn main_stat() {
    unsafe {
        println!("stdin isatty: {}", libc::isatty(0) != 0);
        println!("stdout isatty: {}", libc::isatty(1) != 0);
        println!("stderr isatty: {}", libc::isatty(2) != 0);
        println!("/ isatty: {}", libc::isatty(3) != 0);
    }
}
