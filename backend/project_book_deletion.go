package main

import (
	"errors"
	"os"
	"time"
)

func bookDeletedEntryIndex(book *projectBook, entryID string) int {
	for i := range book.DeletedEntries {
		if book.DeletedEntries[i].ID == entryID {
			return i
		}
	}
	return -1
}

// Call with projectBookMu held. A complete revision is the deletion record;
// original content, source files, media, and previous revisions stay intact.
func setProjectBookEntryDeleted(root *os.Root, book *projectBook, req projectBookRequest) error {
	if req.ExpectedContentVersion == "" {
		return errors.New("Open the current story before deleting or restoring it.")
	}
	deleting := req.Action == "delete-entry"
	activeIndex, deletedIndex := bookVideoEntryIndex(book, req.EntryID), bookDeletedEntryIndex(book, req.EntryID)
	if (deleting && deletedIndex >= 0) || (!deleting && activeIndex >= 0) {
		return nil
	}
	var entry bookEntry
	if deleting && activeIndex >= 0 {
		entry = book.Entries[activeIndex]
	} else if !deleting && deletedIndex >= 0 {
		entry = book.DeletedEntries[deletedIndex]
	} else {
		return errors.New("That Project Book entry was not found.")
	}
	if req.ExpectedContentVersion != bookEntryContentVersion(entry) || (req.ExpectedUpdatedAt != "" && req.ExpectedUpdatedAt != entry.UpdatedAt) {
		return errProjectBookConflict
	}
	if deleting {
		// Stop pending attachments durably before hiding the entry. Otherwise a
		// delete followed by restore could accept an old generation's result.
		if err := supersedeBookVideoBindingsForEdit(root, entry.ID); err != nil {
			return errors.New("Could not save this story's pending video state. The story was not deleted. Try again.")
		}
	}
	now := time.Now().UTC()
	if previous, err := time.Parse(time.RFC3339Nano, entry.UpdatedAt); err == nil && !now.After(previous) {
		now = previous.Add(time.Nanosecond)
	}
	entry.UpdatedAt = now.Format(time.RFC3339Nano)
	entry.DeletedAt = ""
	entry.CanWriteStory = false
	if deleting {
		entry.DeletedAt = entry.UpdatedAt
	}
	if err := saveBookEntry(root, entry); err != nil {
		return err
	}
	entry.ContentVersion = bookEntryContentVersion(entry)
	if deleting {
		book.Entries = append(book.Entries[:activeIndex], book.Entries[activeIndex+1:]...)
		book.DeletedEntries = append(book.DeletedEntries, entry)
	} else {
		book.DeletedEntries = append(book.DeletedEntries[:deletedIndex], book.DeletedEntries[deletedIndex+1:]...)
		book.Entries = append(book.Entries, entry)
	}
	orderProjectBook(book)
	return nil
}
