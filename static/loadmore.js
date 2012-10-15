$(function() {
    (function ($) {

        var showingMore = false;
        var oldestItemDate;

        function showMore() {
            if (showingMore) return;
            showingMore = true;

            var $nav = this;
            $nav.find('.load-more').hide();
            $nav.find('.loading').show();

            $.ajax({
                url: '/stream',
                data: { before: oldestItemDate },
                success: function (data) {
                    // put the posts in the page
                    var $posts = $('#posts');
                    var $urpost = $('#editor .post').clone();
                    $urpost.find('.body').attr('contenteditable', 'false');

                    $.each(data, function(i, val) {
                        oldestItemDate = val.Posted;

                        var posted = new Date(Date.parse(val.Posted));
                        val.PostedTime = $.relatizeDate.strftime(posted, "%i:%M");
                        val.PostedAM = $.relatizeDate.strftime(posted, "%p");
                        val.PostedDate = $.relatizeDate.strftime(posted, "%D %b %Y");

                        $posts.append($('#post-template').mustache(val));
                    });

                    $nav.find('.loading').hide();
                    $nav.find('.load-more').show();
                    showingMore = false;
                },
                error: function (xhr) {
                    // show an error
                }
            });
        }

        $.fn.loadMore = function (datestamp) {
            oldestItemDate = datestamp;

            var moreShower = showMore.bind(this);
            this.find('.load-more button').click(moreShower);

            $(window).scroll(function () {
                if ($(window).scrollTop() > $(document).height() - $(window).height()) {
                    moreShower();
                }
            });
        };

    })(jQuery);
});
