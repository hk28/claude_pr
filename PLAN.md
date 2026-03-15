


# prman

A web application for managing and scraping Perry Rhodan series data from [Perrypedia](https://www.perrypedia.de).

## Overview

prman scrapes metadata from the German science fiction encyclopedia Perrypedia to create organized collections of Perry Rhodan series data. It supports multiple series including:

- **Perry Rhodan (PR Heft)** - The main series
- **Perry Rhodan NEO** - The modern reboot series
- **Silberbände (PRHC)** - Hardcover collections
- Various miniseries (Arkon, Stardust, etc.)

## Features

- **Configurable**: Series configuration stored in yaml files
  - 1 main config file
  - 1 config file per series
  - Example configs can be found in ./example_output
- **Web Scraper**: Extracts title, author, release date, cycle/series info, and descriptions.
  - The website pattern for each issue is part of the config file. Note that the page redirctes to the *real* page.
    - The pattern for series Pr.Heft is: https://www.perrypedia.de/wiki/Quelle:PR%d
      - https://www.perrypedia.de/wiki/Quelle:PR3000 would be redirected to: https://www.perrypedia.de/wiki/Mythos_Erde_(Roman)
- **Metadata Generation**: Creates OPF and JSON metadata files for audiobooks/e-books
- **File Organization**: Automatically organizes files into proper folder structures
  - The audio book folders need to renamed with given pattern from the config file
- **GUI**: Nice and modern looking Website -> as few dependencies as possible
  


### Requirements:
- The tool is used to download/scrape metadata for Perry Rhodan issues (ebook or audio book)
  - This is the workflow
    1) At first the user has to create a new config for this series.
      - The config has parameters for:
        - Series name
        - Download link pattern for the metadata and cover
        - Pattern how the output folder is named
        - Information about release interval to calculate the release date
        - Series length, like number of issues in the series, series is ongoing, series is ended
        - The series has ebooks and/or audiobooks
        - The series has also multiple, generic user defined, boolean states, like "Read", "Consumed", "Released", "Issue available".
          - These states need to be stored and manged to have the current status of each series.
          - Some states can be calculated from the app, others are manually set from the user.
	2) All actions of the app should only be started from user, not automatically.
       1) The user downloads the content to a inbox folder
         - the folder has sub folders ebook and/or audiobook. Inside that are folders the sub series and a folder for each issue.
       2) Scan command: The app scans the inbox for new issues and donwloads new metadata. All metadata should be cached on the disk.
   		- the app also needs to store the current state for each series, including the user defined states.
       3) Update command: The app can calculate which series has new issues and create an output folder for it.
       4) Copy command: copy file/folder from inbox to output folder. This app only copies the files and do not change them. 
		Before the copy process starts, the renaming patterns are shown to the user, so that he can verify it.

	3) Nice looking landing page
  - The landing page has sidebar on the left that shows the actual status of all series, separated for ebook and audio.
    - This includes the gerneic user defined states and also the state of the series itself, like 2 ebook issues are missing.
  - The main section shows an overview of each series and its status.
    - The view of the main section can be changed between different styles, like a file explorer:
      - Big: Shows a card with only the series name and the cover of the first issue for each series. The states are shown as overlays on the cover.
      - Medium: Shows a card with the cover on the left and some metadata and the state on the right
      - Details: Shows the issues for each series as list or table.
  - On top of the main section sits a topbar with features like filter.
    - The main section can be filtered to series, missing issues, ...
    - 
