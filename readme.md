# Bandcamp Sync Utility
I use bandcamp frequently, and wanted to have my library synced on my laptop and pc so I made this program.
If set to run on startup, it will automatically download new items in your bandcamp library in whatever format you specify. This program can also be used to automatically download and extract your whole bandcamp library to a specific place.

## Configuration
the settings.json file has several fields:
- downloads : don't mess with this one probably, its automatically filled out by the program when run.
- format : defines the downloaded audio file format. it will be one of the following
    - aac-hi
    - aiff-lossless
    - alac
    - flac
    - mp3-320
    - mp3-v0
    - vorbis
    - wav
- music_path : the path to the folder where you want your library to be kept in, ended with whatever path separator your os uses
- identity : a cookie bandcamp uses to recognize you; set when you sign in on the website

If the settings file does not exist, an empty one will be automatically be generated when the program is run.

### Getting the identity token
Open the bandcamp website and log in. Then go to your browser's devtools window; on Chrome and Firefox hit Ctrl+Shift+i or Right Click->inspect, then go to the section with cookies. On Firefox it will be under storage->Cookies->https://bandcamp.com. On Chrome it will be under Application->Storage->Cookies->https://bandcamp.com. Copy the Value field of the cookie named 'identity', then paste it into settings.json identity field.

Then you should be ready to go!