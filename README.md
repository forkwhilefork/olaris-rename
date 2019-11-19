### Olaris Rename

A simple tool to automatically rename files based on their information. 

If you want something more powerfull please check out [Filebot](https://www.filebot.net/)

To start scanning give it a `--filepath` argument, this can be a folder or file.

By default it will rename files based purely on the given filenames, alternatively 
you can set `--tmdb-lookup=true`. In this case it will try to look-up actual titles
found in the filename on themoviedb.org, this might result in better names but will
be much slower.


```
  -action string
    	How to act on files, valid options are symlink, hardlink or copy. (default "symlink")
  -dry-run
    	Don't actually modify any files.
  -filepath string
    	Path to scan (can be a folder or file)
  -log-to-file
    	Logs are written to stdout as well as a logfile.
  -movie-folder string
    	Folder where movies should be placed (default "$HOME/media-olaris/Movies")
  -music-folder string
    	Folder where music should be placed (default "$HOME/media-olaris/Music")
  -recursive
    	Scan folders inside of other folders.
  -series-folder string
    	Folder where series should be placed (default "$HOME/media-olaris/TV Shows")
  -tmdb-lookup
    	Should the TMDB be used for better look-up and matching
  -verbose
    	Show debug log information.
```